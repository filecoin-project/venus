package clock

import (
	"github.com/jonboulle/clockwork"
	"time"
)

// Clock provides an interface that packages can use instead of directly
// using the time module, so that chronology-related behavior can be tested.
type Clock interface {
	clockwork.Clock
	NewTimer(d time.Duration) Timer
	AfterFunc(d time.Duration, f func()) Timer
}

type realClock struct {
	clockwork.Clock
}

// NewSystemClock returns a SystemClock that delegates calls to the jonboulee/clockwork package.
// SystemClock is a Clock which simply delegates calls to the actual time
// package; it should be used by packages in production.
func NewSystemClock() Clock {
	return &realClock{Clock: clockwork.NewRealClock()}
}

func (rc *realClock) NewTimer(d time.Duration) Timer {
	return &realTimer{time.NewTimer(d)}
}

func (rc *realClock) AfterFunc(d time.Duration, f func()) Timer {
	return &realTimer{time.AfterFunc(d, f)}
}
