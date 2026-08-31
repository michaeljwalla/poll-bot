package polls

import (
	fh "poll-bot/root/datas/fileheap"
	"poll-bot/root/datas/set"
	"poll-bot/root/datas/sqlite"
	"poll-bot/root/managers/audit"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/discordgo"
)

// TODO
// make a translator func for discordgo.Message bc its unnecessarily large
// for this use case.
// write to external data that isn't the queue. may need to do
// some form of chunking.
const (
	QUEUE_SUBPATH     = "queue.db"
	FINALIZED_SUBPATH = "finalized.db"
	EXPORT_SUBPATH    = "exported.csv"

	//how long to wait before re-checking a poll Discord has not finalized yet
	RETRY_DELAY = time.Duration(30) * time.Second

	WRITE_MAX_BATCH         = 5
	NUM_WORKERS             = 5
	WORKER_TIMEOUT_DURATION = time.Duration(30) * time.Second
)

type snowflake = string

type Message struct {
	ID        snowflake
	ChannelID snowflake
}
type Poll struct {
	Expiry      *time.Time
	Message     Message
	Guild       snowflake          //its an extra api request from Message
	realMessage *discordgo.Message `json:"-"`
}

func ToMessage(msg *discordgo.Message) Message {
	return Message{
		ID:        msg.ID,
		ChannelID: msg.ChannelID,
	}
}

type fetchMode = int

const (
	CACHED = iota
	CACHEDFETCH
	REFETCH
)

func (p *Poll) LiveMessage(mode fetchMode, s *discordgo.Session) (*discordgo.Message, error) {
	if mode == CACHED || (mode == CACHEDFETCH && p.realMessage != nil) {
		return p.realMessage, nil
	}
	//REFETCH \/
	msg, err := s.ChannelMessage(p.Message.ChannelID, p.Message.ID)
	if err != nil {
		return nil, err
	}
	p.realMessage = msg
	return msg, err
}

// must have a Poll
func From(msg *discordgo.Message, guildID snowflake) *Poll {
	return &Poll{
		realMessage: msg,
		Message:     ToMessage(msg),
		Expiry:      msg.Poll.Expiry,
		Guild:       guildID,
	}
}

type PollManager struct {
	path       string
	queue      *fh.FileHeap[Poll]
	finalized  *sqlite.SQLiteDB
	set        set.Set[snowflake] //for quick dupe lookup
	session    atomic.Pointer[discordgo.Session]
	workers    map[string]*resultsWorker
	queueSubCh chan nothing //release flushes values
}

// no-op after first run
func (man *PollManager) SetSession(s *discordgo.Session) bool {
	return man.session.CompareAndSwap(nil, s)
}
func (man *PollManager) HasSession() bool {
	return man.session.Load() != nil
}

// soonest expiry first. A nil expiry means Discord did not tell us when the
// poll closes, so it sinks rather than floats: it must not preempt polls we
// know are due. (The old form returned true for l == r when both were nil,
// which is not a valid ordering for container/heap either.)
func less(l *Poll, r *Poll) bool {
	if l.Expiry == nil {
		return false
	}
	if r.Expiry == nil {
		return true
	}
	return l.Expiry.Before(*r.Expiry)
}

func validate(poll *Poll) bool {
	return poll.Expiry != nil && poll.Guild != ""
}

func (man *PollManager) releaseSubscribers() {
	for {
		select {
		case <-man.queueSubCh:
		default:
			return
		}
	}
}
func (man *PollManager) StartWorker(id string, l *audit.Log) (exists bool) {
	if len(man.workers) >= NUM_WORKERS {
		return true
	} else if _, ok := man.workers[id]; ok {
		return true
	}
	worker, _ := newResultsWorker(man, id, WRITE_MAX_BATCH, l)
	man.workers[id] = worker
	go worker.Start()
	return false
}

func New(path string) (*PollManager, error) {
	table, err := fh.New(path+QUEUE_SUBPATH, less, validate)
	if err != nil {
		return nil, err
	}
	final, err := sqlite.New(path+FINALIZED_SUBPATH, fh.FILE_TIMEOUT)
	if err != nil {
		return nil, err
	}
	if _, err := final.Exec(resultsInitQuery); err != nil {
		return nil, err
	}
	// worker goroutines
	man := PollManager{
		path:       path,
		queue:      table,
		finalized:  final,
		set:        set.New[snowflake](),
		queueSubCh: make(chan nothing),
		workers:    make(map[string]*resultsWorker),
		// worker:
	}

	// worker, _ := newResultsWorker(&man, WRITE_MAX_BATCH)
	// man.worker = worker

	//
	return &man, nil
}

func (man *PollManager) Close() error {
	wg := sync.WaitGroup{}
	for _, worker := range man.workers {
		wg.Go(func() { worker.Stop(WORKER_TIMEOUT_DURATION) }) //nolint
	}
	man.releaseSubscribers()
	wg.Wait()
	return man.queue.Close()
}
func (man *PollManager) Len() int {
	return man.queue.Len()
}
