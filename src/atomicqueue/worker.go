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
	done                chan struct{} // closed exactly once by Stop
	awaitUnblockChannel chan struct{} // buffered(1); poppers signal writer
	subscribers         *subscribers
	states              workerStates
}

func newWorker[T any](queue *AtomicQueue[T], channelSize int, flags int) *worker[T] {
	active := atomic.Bool{}
	active.Store(true)
	return &worker[T]{
		queue:               queue,
		channel:             make(chan *T, channelSize),
		done:                make(chan struct{}),
		awaitUnblockChannel: make(chan struct{}, 1),
		subscribers:         newSubscribers(),
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
}

func (w *worker[T]) Active() bool { return w.states.active.Load() }
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

func (w *worker[T]) Subscribe() {
	w.subscribers.Wait(func() bool {
		return w.states.active.Load() && w.Empty()
	})
}

func (w *worker[T]) ReleaseSubscribers() {
	w.subscribers.Release()
}

func (w *worker[T]) Start() {
	defer w.Stop()
	for {
		if hasFlag(w.states.flags, F_BLOCKFULL) {
			for w.Full() {
				select {
				case <-w.awaitUnblockChannel:
				case <-w.done:
					return
				}
			}
		}
		var next *T
		select {
		case next = <-w.channel:
		case <-w.done:
			return
		}
		if w.Full() {
			// only reachable under F_DROPFULL
			w.states.dropped.Add(1)
			continue
		}
		tail := w.states.tail.Load()
		w.queue.data[w.mask(tail)] = next
		w.states.tail.Store(tail + 1)
		w.subscribers.Release()
	}
}

func (w *worker[T]) Stop() {
	w.states.stopper.Do(func() {
		w.states.active.Store(false)
		close(w.done)
		w.subscribers.Release()
	})
}

func (w *worker[T]) Push(v *T) error {
	select {
	case <-w.done:
		return errors.New("closed")
	default:
	}
	select {
	case w.channel <- v:
		return nil
	case <-w.done:
		return errors.New("closed")
	}
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
		if hasFlag(w.states.flags, F_BLOCKFULL) {
			select {
			case w.awaitUnblockChannel <- struct{}{}:
			default:
			}
		}
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
