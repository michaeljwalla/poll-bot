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

type pollPair = pair.Pair[string, string]
type PollManager struct {
	queue *fh.FileHeap[pollPair]
}

func less(l *pollPair, r *pollPair) bool {
	left, _ := strconv.Atoi(l.First)
	right, _ := strconv.Atoi(r.First)
	return left < right
}
func validate(pair *pollPair) bool {
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

func (man *PollManager) Push(value pollPair) error {
	return man.queue.Push(value)
}
func (man *PollManager) Pop() (pollPair, error) {
	return man.queue.Pop()
}
func (man *PollManager) Peek() (pollPair, bool) {
	return man.queue.Peek()
}

func (man *PollManager) Close() error {
	return man.queue.Close()
}
