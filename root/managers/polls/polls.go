package polls

import (
	fh "poll-bot/root/datas/fileheap"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/discordgo"
)

const (
	QUEUE_SUBPATH   = "queue.txt"
	RATINGS_SUBPATH = "ratings.csv"
	MISC_SUBDIR     = "misc/"
	RATINGS_SUBDIR  = "ratings/"
)

type snowflake = string
type expiry = string

type Poll struct {
	Message snowflake
	Channel snowflake
	Guild   snowflake
	Expiry  *time.Time
}
type PollManager struct {
	queue   *fh.FileHeap[Poll]
	session atomic.Pointer[discordgo.Session]
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
	if _, err := strconv.Atoi(poll.Message); err != nil {
		return false
	}
	if _, err := strconv.Atoi(poll.Channel); err != nil {
		return false
	}
	return poll.Expiry != nil
}
func New(path string) (*PollManager, error) {
	table, err := fh.New(path+QUEUE_SUBPATH, less, validate)
	if err != nil {
		return nil, err
	}

	return &PollManager{
		queue: table,
	}, nil
}

func (man *PollManager) Write() error { return man.queue.SyncWrite() }
func (man *PollManager) Read() error  { return man.queue.SyncRead() }

func (man *PollManager) Push(value Poll) error {
	return man.queue.Push(value)
}
func (man *PollManager) Pop() (Poll, error) {
	return man.queue.Pop()
}
func (man *PollManager) Peek() (Poll, bool) {
	return man.queue.Peek()
}

func (man *PollManager) Close() error {
	return man.queue.Close()
}
func (man *PollManager) Len() int {
	return man.queue.Len()
}
