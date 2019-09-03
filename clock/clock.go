package clock

import (
	"time"
)

// Clock defines an interface for fetching time that may be used instead of the
// time module.
type Clock interface {
	Now() time.Time
	Sleep(d time.Duration)
}

// SystemClock delegates calls to the time package.
type SystemClock struct{}

// NewSystemClock returns a SystemClock that delegates calls to the time package.
func NewSystemClock() *SystemClock {
	return &SystemClock{}
}

// Now returns the current local time.
func (sc *SystemClock) Now() time.Time {
	return time.Now()
}

// Sleep pauses the current goroutine for at least the duration d.
// A negative or zero duration causes Sleep to return immediately.
func (sc *SystemClock) Sleep(d time.Duration) {
	time.Sleep(d)
}
