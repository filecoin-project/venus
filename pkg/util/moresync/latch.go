package moresync

import "sync"

// A Latch allows one or more routines to wait for the completion of a set of events.
// After the events are complete, any waiting routines are released, and subsequent invocations
// of `Wait` return immediately.
//
// It is conceptually similar to a sync.WaitGroup, primary differences being:
// - the count is set once at construction;
// - it is not an error for a latch to receive more than the specified `Done` count.
type Latch struct {
	mutex     sync.Mutex
	remaining uint
	complete  chan struct{}
}

// NewLatch initializes a new latch with an initial `count`.
func NewLatch(count uint) *Latch {
	latch := &Latch{
		remaining: count,
		complete:  make(chan struct{}),
	}
	if count == 0 {
		close(latch.complete)
	}
	return latch
}

// Count returns the latch's current count.
func (latch *Latch) Count() uint {
	latch.mutex.Lock()
	defer latch.mutex.Unlock()
	return latch.remaining
}

// Done reduces the latch's count by one, releasing waiting routines if the resulting count is zero.
// This is a no-op if the count is already zero.
func (latch *Latch) Done() {
	latch.mutex.Lock()
	defer latch.mutex.Unlock()
	if latch.remaining > 0 {
		latch.remaining--
		if latch.remaining == 0 {
			close(latch.complete)
		}
	}
}

// Wait blocks the calling routine until the count reaches zero.
func (latch *Latch) Wait() {
	<-latch.complete
}
