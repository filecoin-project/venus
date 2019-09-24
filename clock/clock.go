package clock

import (
	"time"
)

// Clock provides an interface that packages can use instead of directly
// using the time module, so that chronology-related behavior can be tested
// Adapted from: https://github.com/jonboulle/clockwork
type Clock interface {
	After(d time.Duration) <-chan time.Time
	Sleep(d time.Duration)
	Now() time.Time
	Since(t time.Time) time.Duration

	NewTicker(d time.Duration) Ticker

	NewTimer(d time.Duration) Timer
	AfterFunc(d time.Duration, f func()) Timer
}

type realClock struct{}

func (rc *realClock) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

func (rc *realClock) Sleep(d time.Duration) {
	time.Sleep(d)
}

func (rc *realClock) Now() time.Time {
	return time.Now()
}

func (rc *realClock) Since(t time.Time) time.Duration {
	return rc.Now().Sub(t)
}

func (rc *realClock) NewTicker(d time.Duration) Ticker {
	return &realTicker{time.NewTicker(d)}
}

func (rc *realClock) NewTimer(d time.Duration) Timer {
	return &realTimer{time.NewTimer(d)}
}

func (rc *realClock) AfterFunc(d time.Duration, f func()) Timer {
	return &realTimer{time.AfterFunc(d, f)}
}

// NewSystemClock returns a Clock that delegates calls to the actual time
// package; it should be used by packages in production.
func NewSystemClock() Clock {
	return &realClock{}
}
