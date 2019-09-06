package testhelpers

import (
	"time"

	"github.com/jonboulle/clockwork"
)

// FakeSystemClock wrapps the jonboulle/clockwork.FakeClock interface.
// FakeSystemClock provides an interface for a clock which can be
// manually advanced through time.
type FakeSystemClock interface {
	clockwork.FakeClock
}

// NewFakeSystemClock returns a FakeSystemClock initialised at the given time.Time.
func NewFakeSystemClock(n time.Time) FakeSystemClock {
	return clockwork.NewFakeClockAt(n)
}
