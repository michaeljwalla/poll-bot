package atomicqueue

import (
	"errors"
	"sync"
	"sync/atomic"
)

// flags
const (
	F_BLOCKFULL = 1 << 0
	F_DROPFULL  = 1 << 1
)

func hasFlag(cmp int, f int) bool {
	return cmp&f == f
}

type workerStates struct {
	stopper     *sync.Once
	active      *atomic.Bool
	empty       *atomic.Bool
	front, end  *atomic.Int32
	dropped     *atomic.Int32
	channelSize int
	flags       int
}
type worker[T any] struct {
	queue               *AtomicQueue[T]
	channel             chan *T
	awaitUnblockChannel chan byte
	states              workerStates
}

func flushChannel[T any](ch chan T) {
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return // closed
			}
		default:
			return
		}
	}
}
func newWorker[T any](queue *AtomicQueue[T], channelSize int, flags int) *worker[T] {
	active := atomic.Bool{}
	active.Store(true)
	empty := atomic.Bool{}
	empty.Store(true)
	worker := worker[T]{
		queue:               queue,
		channel:             make(chan *T, channelSize),
		awaitUnblockChannel: make(chan byte),
		states: workerStates{
			active:      &active,
			channelSize: channelSize,
			flags:       flags,
			empty:       &empty,
			stopper:     &sync.Once{},
			front:       &atomic.Int32{},
			end:         &atomic.Int32{},
			dropped:     &atomic.Int32{},
		},
	}
	return &worker
}
func (w *worker[T]) Active() bool {
	return w.states.active.Load()
}
func (w *worker[T]) Length() int {
	if w.Empty() {
		return 0
	}

	front, end := int(w.states.front.Load()), int(w.states.end.Load())
	if front == end {
		return int(w.queue.size)
	}

	if front > end {
		return w.queue.size - front + end + 1
	}
	return int(end-front) + 1
}
func (w *worker[T]) Full() bool {
	return w.states.front.Load() == w.states.end.Load() && !w.states.empty.Load()
}
func (w *worker[T]) Empty() bool {
	return w.states.empty.Load()
}
func (w *worker[T]) next(idx int32) int32 {
	return (idx + 1) & (int32(w.queue.size) - 1)
}
func (w *worker[T]) Dropped() int {
	return int(w.states.dropped.Load())
}

func (w *worker[T]) Start() {
	for {
		if w.Full() && hasFlag(w.states.flags, F_BLOCKFULL) {
			_, ok := <-w.awaitUnblockChannel
			flushChannel(w.awaitUnblockChannel)
			if !ok {
				break
			}
		}

		next, ok := <-w.channel
		if !ok || !w.states.active.Load() {
			break
		} else if w.Full() {
			w.states.dropped.Add(1)
			continue
		}
		end := w.states.end.Load()
		//must increment end afterwards so threads don't read mid write
		w.queue.data[end] = next
		w.states.end.Store(w.next(end))
		w.states.empty.Store(false)
	}
	w.Stop() //idempotent sooo
}
func (w *worker[T]) Stop() {
	w.states.stopper.Do(func() {
		w.states.active.Store(false)
		close(w.awaitUnblockChannel)
		close(w.channel)
	})
}
func (w *worker[T]) Push(v *T) error {
	if !w.states.active.Load() {
		return errors.New("Empty")
	}
	w.channel <- v
	return nil
}
func (w *worker[T]) tryPop(output *T) (success bool, empty bool) {
	if w.Empty() {
		empty = true
		return
	}
	idx := w.states.front.Load()
	val := *w.queue.data[idx]
	if w.states.front.CompareAndSwap(idx, w.next(idx)) {
		//won the race
		success = true
		*output = val
		return
	}
	return
}
func (w *worker[T]) Pop() (value *T, err error) {
	var out T
	for {
		success, empty := w.tryPop(&out)
		if success {
			break
		} else if empty {
			value = nil
			err = errors.New("Empty")
			return
		}
	}
	value = &out
	return
}
