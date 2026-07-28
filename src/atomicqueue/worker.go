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
	head, tail  *atomic.Uint64 // ring index is (x & (size-1))
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
	worker := worker[T]{
		queue:               queue,
		channel:             make(chan *T, channelSize),
		awaitUnblockChannel: make(chan byte),
		states: workerStates{
			active:      &active,
			channelSize: channelSize,
			flags:       flags,
			stopper:     &sync.Once{},
			head:        &atomic.Uint64{},
			tail:        &atomic.Uint64{},
			dropped:     &atomic.Int32{},
		},
	}
	return &worker
}
func (w *worker[T]) Active() bool {
	return w.states.active.Load()
}
func (w *worker[T]) Length() int {
	return int(w.states.tail.Load() - w.states.head.Load())
}
func (w *worker[T]) Full() bool {
	return w.states.tail.Load()-w.states.head.Load() >= uint64(w.queue.size)
}
func (w *worker[T]) Empty() bool {
	return w.states.head.Load() == w.states.tail.Load()
}
func (w *worker[T]) mask(idx uint64) uint64 {
	return idx & uint64(w.queue.size-1)
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
		tail := w.states.tail.Load()
		w.queue.data[w.mask(tail)] = next
		w.states.tail.Store(tail + 1)
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
	head := w.states.head.Load()
	tail := w.states.tail.Load()
	if head == tail {
		empty = true
		return
	}
	val := *w.queue.data[w.mask(head)]

	if w.states.head.CompareAndSwap(head, head+1) {
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
