package clock

import (
	"github.com/jonboulle/clockwork"
)

// Clock provides an interface that packages can use instead of directly
// using the time module, so that chronology-related behavior can be tested.
type Clock interface {
	clockwork.Clock
}

// SystemClock is a wrapper around the jonboulle/clockwork.Clock interface
// Clock provides an interface that packages can use instead of directly
// using the time module, so that chronology-related behavior can be tested.
type SystemClock struct {
	clockwork.Clock
}

// NewSystemClock returns a SystemClock that delegates calls to the jonboulee/clockwork package.
func NewSystemClock() *SystemClock {
	return &SystemClock{
		Clock: clockwork.NewRealClock(),
	}
}
