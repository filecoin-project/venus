package testhelpers

import (
	"github.com/jonboulle/clockwork"
	"time"
)

// FakeSystemClock wrapps the jonboulle/clockwork.FakeClock interface.
// FakeSystemClock provides an interface for a clock which can be
// manually advanced through time.
type FakeSystemClock struct {
	clockwork.FakeClock
}

// NewFakeSystemClock returns a FakeSystemClock initialised at the given time.Time.
func NewFakeSystemClock(n time.Time) *FakeSystemClock {
	return &FakeSystemClock{
		FakeClock: clockwork.NewFakeClockAt(n),
	}
}
