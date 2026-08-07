package polls

import (
	fh "poll-bot/root/datas/fileheap"
	"poll-bot/root/datas/pair"
	"strconv"
)

const (
	QUEUE_SUBPATH   = "queue.txt"
	RATINGS_SUBPATH = "ratings.csv"
	MISC_SUBDIR     = "misc/"
)

type snowflake = string
type expiry = string

type PollPair = pair.Pair[snowflake, expiry]
type PollManager struct {
	queue *fh.FileHeap[PollPair]
}

func less(l *PollPair, r *PollPair) bool {
	left, _ := strconv.Atoi(l.First)
	right, _ := strconv.Atoi(r.First)
	return left < right
}
func validate(pair *PollPair) bool {
	_, err := strconv.Atoi(pair.First)
	return err == nil
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

func (man *PollManager) Push(value PollPair) error {
	return man.queue.Push(value)
}
func (man *PollManager) Pop() (PollPair, error) {
	return man.queue.Pop()
}
func (man *PollManager) Peek() (PollPair, bool) {
	return man.queue.Peek()
}

func (man *PollManager) Close() error {
	return man.queue.Close()
}
func (man *PollManager) Len() int {
	return man.queue.Len()
}
