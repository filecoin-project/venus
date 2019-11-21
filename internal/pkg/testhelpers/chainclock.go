package testhelpers

import (
	"sync"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
)

// FakeChainClock is an interface for a clock that represents epochs of the protocol.
type FakeChainClock interface {
	FakeClock

	EpochAtTime(t time.Time) clock.ChainEpoch
}

type fakeChainClock struct {
	epochMu sync.Mutex
	epoch   uint64

	FakeClock
}

// NewFakeChainClock returns a FakeChainClock.
func NewFakeChainClock(chainEpoch uint64, wallTime time.Time) FakeChainClock {
	return &fakeChainClock{
		epoch:     chainEpoch,
		FakeClock: NewFakeClock(wallTime),
	}
}

// EpochAtTime returns the ChainEpoch held by the fakeChainClock.
func (fcc *fakeChainClock) EpochAtTime(t time.Time) clock.ChainEpoch {
	fcc.epochMu.Lock()
	defer fcc.epochMu.Unlock()
	return clock.ChainEpoch(fcc.epoch)
}

// SetChainEpoch sets the FakeChainClock's ChainEpoch to `epoch`.
func (fcc *fakeChainClock) SetChainEpoch(epoch uint64) {
	fcc.epochMu.Lock()
	defer fcc.epochMu.Unlock()
	fcc.epoch = epoch
}
