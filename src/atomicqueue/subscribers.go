package atomicqueue

import "sync"

// so other goroutines can sleep-wait for an item from the queue
type subscribers struct {
	mu     sync.Mutex
	ch     chan struct{}
	closed bool
}

func newSubscribers() *subscribers {
	return &subscribers{ch: make(chan struct{})}
}

func (s *subscribers) Wait(shouldWait func() bool) {
	s.mu.Lock()
	if !shouldWait() {
		s.mu.Unlock()
		return
	}
	if s.closed {
		s.ch = make(chan struct{})
		s.closed = false
	}
	ch := s.ch
	s.mu.Unlock()
	<-ch
}

func (s *subscribers) Release() {
	s.mu.Lock()
	if !s.closed {
		close(s.ch)
		s.closed = true
	}
	s.mu.Unlock()
}
