package clock

import (
	"context"
	"time"

	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/venus/internal/pkg/specactors/builtin"
)

// DefaultEpochDuration is the default duration of epochs
const DefaultEpochDuration = builtin.EpochDurationSeconds * time.Second

// DefaultPropagationDelay is the default time to await for blocks to arrive before mining
const DefaultPropagationDelay = 6 * time.Second

// ChainEpochClock is an interface for a clock that represents epochs of the protocol.
type ChainEpochClock interface {
	EpochDuration() time.Duration
	EpochAtTime(t time.Time) abi.ChainEpoch
	EpochRangeAtTimestamp(t uint64) (abi.ChainEpoch, abi.ChainEpoch)
	StartTimeOfEpoch(e abi.ChainEpoch) time.Time
	WaitForEpoch(ctx context.Context, e abi.ChainEpoch)
	WaitForEpochPropDelay(ctx context.Context, e abi.ChainEpoch)
	WaitNextEpoch(ctx context.Context) abi.ChainEpoch
	Clock
}

// chainClock is a clock that represents epochs of the protocol.
type chainClock struct {
	// The time of the first block. EpochClock counts up from there.
	genesisTime time.Time
	// The fixed time length of the epoch window
	epochDuration time.Duration
	// propDelay is the time between the start of the epoch and the start
	// of mining for the subsequent epoch. This delay provides time for
	// blocks from the previous epoch to arrive.
	propDelay time.Duration

	Clock
}

// NewChainClock returns a ChainEpochClock wrapping a default clock.Clock
func NewChainClock(genesisTime uint64, blockTime time.Duration, propDelay time.Duration) ChainEpochClock {
	return NewChainClockFromClock(genesisTime, blockTime, propDelay, NewSystemClock())
}

// NewChainClockFromClock returns a ChainEpochClock wrapping the provided
// clock.Clock
func NewChainClockFromClock(genesisSeconds uint64, blockTime time.Duration, propDelay time.Duration, c Clock) ChainEpochClock {
	gt := time.Unix(int64(genesisSeconds), 0)
	return &chainClock{
		genesisTime:   gt,
		epochDuration: blockTime,
		propDelay:     propDelay,
		Clock:         c,
	}
}

func (cc *chainClock) EpochDuration() time.Duration {
	return cc.epochDuration
}

// EpochAtTime returns the ChainEpoch corresponding to t.
// It first subtracts genesisTime, then divides by epochDuration
// and returns the resulting number of epochs.
func (cc *chainClock) EpochAtTime(t time.Time) abi.ChainEpoch {
	difference := t.Sub(cc.genesisTime)
	epochs := difference / cc.epochDuration
	return abi.ChainEpoch(epochs)
}

// EpochRangeAtTimestamp returns the possible epoch number range a given
// unix second timestamp value can validly belong to.  This method can go
// away once integration tests work well enough to not require subsecond
// block times.
func (cc *chainClock) EpochRangeAtTimestamp(seconds uint64) (abi.ChainEpoch, abi.ChainEpoch) {
	earliest := time.Unix(int64(seconds), 0)
	first := cc.EpochAtTime(earliest)
	latest := earliest.Add(time.Second)
	last := cc.EpochAtTime(latest)
	return first, last
}

// StartTimeOfEpoch returns the start time of the given epoch.
func (cc *chainClock) StartTimeOfEpoch(e abi.ChainEpoch) time.Time {
	addedTime := cc.genesisTime.Add(cc.epochDuration * time.Duration(e))
	return addedTime
}

// WaitNextEpoch returns after the next epoch occurs, or ctx is done.
func (cc *chainClock) WaitNextEpoch(ctx context.Context) abi.ChainEpoch {
	currEpoch := cc.EpochAtTime(cc.Now())
	nextEpoch := currEpoch + 1
	cc.WaitForEpoch(ctx, nextEpoch)
	return nextEpoch
}

// WaitForEpoch returns when an epoch is due to start, or ctx is done.
func (cc *chainClock) WaitForEpoch(ctx context.Context, e abi.ChainEpoch) {
	cc.waitForEpochOffset(ctx, e, 0)
}

// WaitForEpochPropDelay returns propDelay time after the start of the epoch, or when ctx is done.
func (cc *chainClock) WaitForEpochPropDelay(ctx context.Context, e abi.ChainEpoch) {
	cc.waitForEpochOffset(ctx, e, cc.propDelay)
}

// waitNextEpochOffset returns when time is offset past the start of the epoch, or ctx is done.
func (cc *chainClock) waitForEpochOffset(ctx context.Context, e abi.ChainEpoch, offset time.Duration) {
	targetTime := cc.StartTimeOfEpoch(e).Add(offset)
	nowB4 := cc.Now()
	waitDur := targetTime.Sub(nowB4)
	if waitDur > 0 {
		newEpochCh := cc.After(waitDur)
		select {
		case <-newEpochCh:
		case <-ctx.Done():
		}
	}
}
