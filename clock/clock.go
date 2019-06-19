package clock

import (
	"time"
)

// Clock defines an interface for fetching time that may be used instead of the
// time module.
type Clock interface {
	Now() time.Time
}

// SystemClock delegates calls to the time package.
type SystemClock struct{}

// NewSystemClock returns a SystemClock that delegates calls to the time package.
func NewSystemClock() *SystemClock {
	return &SystemClock{}
}

// Now returns the current local time.
func (bc *SystemClock) Now() time.Time {
	return time.Now()
}
