package clock

import (
	"github.com/jonboulle/clockwork"
)

// Ticker provides an interface which can be used instead of directly
// using the ticker within the time module. The real-time ticker t
// provides ticks through t.C which becomes now t.Chan() to make
// this channel requirement definable in this interface.
// Ticker is an alias for clockwork.Ticker
type Ticker = clockwork.Ticker

// Clock provides an interface that packages can use instead of directly
// using the time module, so that chronology-related behavior can be tested.
type Clock interface {
	clockwork.Clock
}

// NewSystemClock returns a SystemClock that delegates calls to the jonboulee/clockwork package.
// SystemClock is a Clock which simply delegates calls to the actual time
// package; it should be used by packages in production.
func NewSystemClock() Clock {
	return clockwork.NewRealClock()
}
