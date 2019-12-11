package clock

import (
	"fmt"
	"time"
)

// DefaultBlockTime is the default duration of epochs
const DefaultBlockTime = 15 * time.Second

// ChainEpochClock is an interface for a clock that represents epochs of the protocol.
type ChainEpochClock interface {
	EpochAtTime(t time.Time) int64
	StartTimeOfEpoch(e uint64) time.Time
	Clock
}

// chainClock is a clock that represents epochs of the protocol.
type chainClock struct {
	// GenesisTime is the time of the first block. EpochClock counts
	// up from there.
	GenesisTime time.Time
	// EpochDuration is the time an epoch takes
	EpochDuration time.Duration

	Clock
}

// NewChainClock returns a ChainEpochClock wrapping a default clock.Clock
func NewChainClock(genesisTime uint64, blockTime time.Duration) ChainEpochClock {
	return NewChainClockFromClock(genesisTime, blockTime, NewSystemClock())
}

// NewChainClockFromClock returns a ChainEpochClock wrapping the provided
// clock.Clock
func NewChainClockFromClock(genesisTime uint64, blockTime time.Duration, c Clock) ChainEpochClock {
	gt := time.Unix(int64(genesisTime), int64(genesisTime%1000000000))
	fmt.Printf("gen time: h%d-m%d-s%d-m%d\n", gt.Hour(), gt.Minute(), gt.Second(), gt.Nanosecond()/1000000)
	return &chainClock{
		GenesisTime:   gt,
		EpochDuration: blockTime,
		Clock:         c,
	}
}

// EpochAtTime returns the ChainEpoch corresponding to t.
// It first subtracts GenesisTime, then divides by EpochDuration
// and returns the resulting number of epochs.
func (cc *chainClock) EpochAtTime(t time.Time) int64 {
	difference := t.Sub(cc.GenesisTime)
	epochs := difference / cc.EpochDuration
	return int64(epochs)
}

// StartTimeOfEpoch returns the start time of the given epoch.
func (cc *chainClock) StartTimeOfEpoch(e uint64) time.Time {
	addedTime := cc.GenesisTime.Add(cc.EpochDuration * time.Duration(int64(e)))
	//	fmt.Printf("epoch %d start time: h%d-m%d-s%d-m%d\n", e, addedTime.Hour(), addedTime.Minute(), addedTime.Second(), addedTime.Nanosecond()/1000000)
	//	fmt.Printf("duration: %s\n", cc.EpochDuration*time.Duration(int64(e)))
	return addedTime
}
