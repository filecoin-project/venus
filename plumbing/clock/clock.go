package clock

import (
	"time"
)

// DefaultBlockTime is the estimated proving period time.
// We define this so that we can fake mining in the current incomplete system.
const DefaultBlockTime = 30 * time.Second

// BlockClock defines an interface for fetching the block time.
type BlockClock interface {
	BlockTime() time.Duration
}

// DefaultBlockClock implements BlockClock and can be used to get the
// block time.
type DefaultBlockClock struct {
	blockTime time.Duration
}

// NewDefaultBlockClock returns a DefaultBlockClock. It can be used to
// get the value of block time. NewDefaultBlockClock uses DefaltBlockTime
// as the block time.
func NewDefaultBlockClock() *DefaultBlockClock {
	return &DefaultBlockClock{
		blockTime: DefaultBlockTime,
	}
}

// NewConfiguredBlockClock returns a DefaultBlockClock with the provided
// blockTime.
func NewConfiguredBlockClock(blockTime time.Duration) *DefaultBlockClock {
	return &DefaultBlockClock{
		blockTime: blockTime,
	}
}

// BlockTime returns the block time DefaultBlockClock was configured to use.
func (bc *DefaultBlockClock) BlockTime() time.Duration {
	return bc.blockTime
}
