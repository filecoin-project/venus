package clock

import (
	"time"
)

// Clock defines an interface for fetching time that may be used instead of the
// time module.
type Clock interface {
	Now() time.Time
}

// BlockClock delegates calls to the time package.
type BlockClock struct{}

// NewBlockClock returns a BlockClock that delegates calls to the time package.
func NewBlockClock() *BlockClock {
	return &BlockClock{}
}

// Now returns the current local time.
func (bc *BlockClock) Now() time.Time {
	return time.Now()
}

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
