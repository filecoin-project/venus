package clock

import (
	"time"
)

// EpochDuration is a constant that represents the UTC time duration
// of a blockchain epoch.
const EpochDuration = EpochUnits * time.Duration(EpochCount)

// EpochCount is the number of instances in an Epoch.
const EpochCount = int64(15)

// EpochUnits is the units an Epoch is measured in.
const EpochUnits = time.Second

// ChainEpochClock is an interface for a clock that represents epochs of the protocol.
type ChainEpochClock interface {
	EpochAtTime(t time.Time) int64
	Clock
}

// chainClock is a clock that represents epochs of the protocol.
type chainClock struct {
	// GenesisTime is the time of the first block. EpochClock counts
	// up from there.
	GenesisTime time.Time

	Clock
}

// NewChainClock returns a ChainEpochClock.
func NewChainClock(genesisTime uint64) ChainEpochClock {
	gt := time.Unix(int64(genesisTime), 0)
	return &chainClock{GenesisTime: gt, Clock: NewSystemClock()}
}

// EpochAtTime returns the ChainEpoch corresponding to t.
// It first subtracts GenesisTime, then divides by EpochDuration
// and returns the resulting number of epochs.
func (cc *chainClock) EpochAtTime(t time.Time) int64 {
	difference := t.Sub(cc.GenesisTime)
	epochs := difference / EpochDuration
	return int64(epochs)
}
