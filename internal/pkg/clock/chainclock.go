package clock

import (
	"context"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// DefaultEpochDuration is the default duration of epochs
const DefaultEpochDuration = 15 * time.Second

// ChainEpochClock is an interface for a clock that represents epochs of the protocol.
type ChainEpochClock interface {
	EpochAtTime(t time.Time) *types.BlockHeight
	EpochRangeAtTimestamp(t uint64) (*types.BlockHeight, *types.BlockHeight)
	StartTimeOfEpoch(e *types.BlockHeight) time.Time
	WaitNextEpoch(ctx context.Context) *types.BlockHeight
	Clock
}

// chainClock is a clock that represents epochs of the protocol.
type chainClock struct {
	// GenesisTime is the time of the first block. EpochClock counts
	// up from there.
	GenesisTime time.Time
	// EpochDuration is the fixed time length of the epoch window
	EpochDuration time.Duration

	Clock
}

// NewChainClock returns a ChainEpochClock wrapping a default clock.Clock
func NewChainClock(genesisTime uint64, blockTime time.Duration) ChainEpochClock {
	return NewChainClockFromClock(genesisTime, blockTime, NewSystemClock())
}

// NewChainClockFromClock returns a ChainEpochClock wrapping the provided
// clock.Clock
func NewChainClockFromClock(genesisSeconds uint64, blockTime time.Duration, c Clock) ChainEpochClock {
	gt := time.Unix(int64(genesisSeconds), 0)
	return &chainClock{
		GenesisTime:   gt,
		EpochDuration: blockTime,
		Clock:         c,
	}
}

// EpochAtTime returns the ChainEpoch corresponding to t.
// It first subtracts GenesisTime, then divides by EpochDuration
// and returns the resulting number of epochs.
func (cc *chainClock) EpochAtTime(t time.Time) *types.BlockHeight {
	difference := t.Sub(cc.GenesisTime)
	epochs := difference / cc.EpochDuration
	return types.NewBlockHeight(uint64(epochs))
}

// EpochRangeAtTimestamp returns the possible epoch number range a given
// unix second timestamp value can validly belong to.  This method can go
// away once integration tests work well enough to not require subsecond
// block times.
func (cc *chainClock) EpochRangeAtTimestamp(seconds uint64) (*types.BlockHeight, *types.BlockHeight) {
	earliest := time.Unix(int64(seconds), 0)
	first := cc.EpochAtTime(earliest)
	latest := earliest.Add(time.Second)
	last := cc.EpochAtTime(latest)
	return first, last
}

// StartTimeOfEpoch returns the start time of the given epoch.
func (cc *chainClock) StartTimeOfEpoch(e *types.BlockHeight) time.Time {
	bigE := e.AsBigInt()
	addedTime := cc.GenesisTime.Add(cc.EpochDuration * time.Duration(bigE.Int64()))
	return addedTime
}

// WaitNextEpoch returns after the next epoch occurs or ctx is done.
func (cc *chainClock) WaitNextEpoch(ctx context.Context) *types.BlockHeight {
	currEpoch := cc.EpochAtTime(cc.Now())
	nextEpoch := currEpoch.Add(types.NewBlockHeight(uint64(1)))
	nextEpochStart := cc.StartTimeOfEpoch(nextEpoch)
	nowB4 := cc.Now()
	waitDur := nextEpochStart.Sub(nowB4)
	newEpochCh := cc.After(waitDur)
	select {
	case <-newEpochCh:
	case <-ctx.Done():
	}

	return nextEpoch
}
