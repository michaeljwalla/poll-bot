package atomicqueue

import (
	"errors"
	"fmt"
)

const (
	defaultBufferSize int = 1 << 8 //256
)

type AtomicQueue[T any] struct {
	data   []*T
	size   int
	flags  int
	worker *worker[T]
}

// set size->0 for default; set channelSize->0 to equal size
func New[T any](size int, channelSize int, flags int) (queue *AtomicQueue[T], err error) {
	if size < 0 || channelSize < 0 {
		err = errors.New("Buffer size / channel size should be nonnegative values")
		return
	}
	if size%2 != 0 {
		err = fmt.Errorf("Buffer size should be power of two (specify 0 for default %d)", defaultBufferSize)
		return
	} else if size == 0 && defaultBufferSize > 0 {
		size = defaultBufferSize
	} else {
		err = errors.New("Size / default buffer size should be positive")
		return
	}
	if channelSize == 0 {
		channelSize = size
	}

	if hasFlag(flags, F_BLOCKFULL) == hasFlag(flags, F_DROPFULL) {
		err = fmt.Errorf("Specify either F_BLOCKFULL / F_DROPFULL")
		return
	}
	//
	queue = &AtomicQueue[T]{
		data: make([]*T, size+1),
		size: size,
	}
	queue.worker = newWorker(queue, channelSize, flags)
	go queue.worker.Start()
	return
}

func (q *AtomicQueue[T]) Push(value T) error {
	return q.worker.Push(&value)
}
func (q *AtomicQueue[T]) Pop() (value T, err error) {
	v, err := q.worker.Pop()
	if err != nil {
		value = *v
	}
	return
}

func (q *AtomicQueue[T]) Active() bool {
	return q.worker.Active()
}

func (q *AtomicQueue[T]) Length() int {
	return q.worker.Length()
}
func (q *AtomicQueue[T]) Size() int {
	return q.size
}
func (q *AtomicQueue[T]) Full() bool {
	return q.worker.Full()
}
func (q *AtomicQueue[T]) Empty() bool {
	return q.worker.Empty()
}

// can no longer push to queue
func (q *AtomicQueue[T]) Close() {
	q.worker.Stop()
}

func (q *AtomicQueue[T]) Dropped() int {
	return q.worker.Dropped()
}
