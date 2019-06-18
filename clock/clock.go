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
