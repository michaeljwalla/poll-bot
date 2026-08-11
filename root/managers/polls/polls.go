package polls

import (
	fh "poll-bot/root/datas/fileheap"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/discordgo"
)

const (
	QUEUE_SUBPATH   = "queue.json"
	RATINGS_SUBPATH = "ratings.csv"
	MISC_SUBDIR     = "misc/"
	RATINGS_SUBDIR  = "ratings/"
)

type snowflake = string

type Poll struct {
	Expiry  *time.Time
	Message *discordgo.Message
	Guild   snowflake //its an extra api request from Message
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
	return poll.Expiry != nil && poll.Message != nil && poll.Guild != ""
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

// pushing once is O(logn)
//
// pushing multiple initiates re-heapify O(n)
func (man *PollManager) Push(value ...Poll) error {
	if len(value) == 0 {
		return nil
	} else if len(value) == 1 {
		return man.queue.Push(value[0])
	}
	return man.queue.Merge(value...)
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
