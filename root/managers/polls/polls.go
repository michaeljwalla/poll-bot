package polls

import (
	fh "poll-bot/root/datas/fileheap"
	"poll-bot/root/datas/set"
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
	set     set.Set[snowflake] //for quick dupe lookup
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
		set:   set.New[snowflake](),
	}, nil
}
