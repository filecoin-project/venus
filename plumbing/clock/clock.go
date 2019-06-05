package clock

import (
	"time"
)

// BlockClock defines an interface for fetching the block time.
type BlockClock interface {
	BlockTime() time.Duration
	EpochSeconds() int64
}

// DefaultBlockClock implements BlockClock and can be used to get the
// block time.
type DefaultBlockClock struct {
	blockTime time.Duration
}

// NewDefaultBlockClock returns a DefaultBlockClock. It can be used to
// get the value of block time.
func NewDefaultBlockClock(t time.Duration) *DefaultBlockClock {
	return &DefaultBlockClock{
		blockTime: t,
	}
}

// BlockTime returns the block time DefaultBlockClock was configured to use.
func (bc *DefaultBlockClock) BlockTime() time.Duration {
	return bc.blockTime
}

// EpochSeconds returns Unix time, the number of seconds elapsed
// since January 1, 1970 UTC. The result does not depend on the
// location associated with DefaultBlockClock.
func (bc *DefaultBlockClock) EpochSeconds() int64 {
	return time.Now().Unix()
}
