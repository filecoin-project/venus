package testhelpers

import (
	"time"
)

// MockClock returns a mocked clock implementation that may be manually
// set for testing things related to time.
type MockClock struct {
	now time.Time
}

// NewMockClock returns a mocked clock implementation that may be manually
// set for testing things related to time.
func NewMockClock(n time.Time) *MockClock {
	return &MockClock{
		now: n,
	}
}

// Now returns the current value of the MockClock.
func (mc *MockClock) Now() time.Time {
	return mc.now
}

// Set sets the current time value of the MockClock.
func (mc *MockClock) Set(t time.Time) {
	mc.now = t
}
