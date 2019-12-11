package testhelpers

import (
	"time"
)

// FakeChainClock is an interface for a clock that represents epochs of the protocol.
type FakeChainClock interface {
	FakeClock

	EpochAtTime(t time.Time) int64
	StartTimeOfEpoch(e uint64) time.Time
}

type fakeChainClock struct {
	epoch int64

	FakeClock
}

// NewFakeChainClock returns a FakeChainClock.
func NewFakeChainClock(chainEpoch int64, wallTime time.Time) FakeChainClock {
	return &fakeChainClock{
		epoch:     chainEpoch,
		FakeClock: NewFakeClock(wallTime),
	}
}

// EpochAtTime returns the ChainEpoch held by the fakeChainClock.
func (fcc *fakeChainClock) EpochAtTime(t time.Time) int64 {
	return fcc.epoch
}

// StartTimeOfEpoch returns the current time
func (fcc *fakeChainClock) StartTimeOfEpoch(e uint64) time.Time {
	return fcc.Now()
}

// SetChainEpoch sets the FakeChainClock's ChainEpoch to `epoch`.
func (fcc *fakeChainClock) SetChainEpoch(epoch int64) {
	fcc.epoch = epoch
}
