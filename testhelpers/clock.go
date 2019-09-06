package testhelpers

import (
	"time"
)

// FakeSystemClock returns a mocked clock implementation that may be manually
// set for testing things related to time.
type FakeSystemClock struct {
	now time.Time
}

// NewFakeSystemClock returns a mocked clock implementation that may be manually
// set for testing things related to time.
func NewFakeSystemClock(n time.Time) *FakeSystemClock {
	return &FakeSystemClock{
		now: n,
	}
}

// Now returns the current value of the FakeSystemClock.
func (fc *FakeSystemClock) Now() time.Time {
	return fc.now
}

// Sleep returns immediately.
func (fc *FakeSystemClock) Sleep(d time.Duration) {
	return
}

// Set sets the current time value of the FakeSystemClock.
func (fc *FakeSystemClock) Set(t time.Time) {
	fc.now = t
}
