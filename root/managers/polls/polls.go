package polls

import (
	fh "poll-bot/root/datas/fileheap"
	"poll-bot/root/datas/set"
	"poll-bot/root/datas/sqlite"
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
)

type snowflake = string

type message struct {
	ID        snowflake
	ChannelID snowflake
}
type Poll struct {
	Expiry      *time.Time
	Message     message
	Guild       snowflake //its an extra api request from Message
	realMessage *discordgo.Message
}

func ToMessage(msg *discordgo.Message) message {
	return message{
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
	queue     *fh.FileHeap[Poll]
	finalized *sqlite.SQLiteDB
	set       set.Set[snowflake] //for quick dupe lookup
	session   atomic.Pointer[discordgo.Session]
}

// no-op after first run
func (man *PollManager) SetSession(s *discordgo.Session) bool {
	return man.session.CompareAndSwap(nil, s)
}
func (man *PollManager) HasSession() bool {
	return man.session.Load() != nil
}
func less(l *Poll, r *Poll) bool {
	return l.Expiry == nil || (r.Expiry != nil && l.Expiry.Before(*r.Expiry))
}

func validate(poll *Poll) bool {
	return poll.Expiry != nil && poll.Guild != ""
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
	return &PollManager{
		queue:     table,
		finalized: final,
		set:       set.New[snowflake](),
	}, nil
}
