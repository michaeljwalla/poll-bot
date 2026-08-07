package polls

import (
	fh "poll-bot/root/datas/fileheap"
	"poll-bot/root/datas/pair"
	"strconv"
)

type pollPair = pair.Pair[string, string]
type PollQueue struct {
	data *fh.FileHeap[pollPair]
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
func New(path string) (*PollQueue, error) {
	table, err := fh.New(path, less, validate)
	if err != nil {
		return nil, err
	}

	return &PollQueue{
		data: table,
	}, nil
}

func (queue *PollQueue) Write() error { return queue.data.SyncWrite() }
func (queue *PollQueue) Read() error  { return queue.data.SyncRead() }

func (queue *PollQueue) Push(value pollPair) error {
	return queue.data.Push(value)
}
func (queue *PollQueue) Pop() (pollPair, error) {
	return queue.data.Pop()
}
func (queue *PollQueue) Peek() (pollPair, bool) {
	return queue.data.Peek()
}
