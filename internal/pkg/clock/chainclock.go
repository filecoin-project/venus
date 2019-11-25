package clock

import (
	"time"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// EpochDuration is a constant that represents the UTC time duration
// of a blockchain epoch.
var EpochDuration = time.Second * 15

// ChainEpochClock is an interface for a clock that represents epochs of the protocol.
type ChainEpochClock interface {
	EpochAtTime(t time.Time) *types.BlockHeight
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
func (cc *chainClock) EpochAtTime(t time.Time) *types.BlockHeight {
	difference := t.Sub(cc.GenesisTime)
	epochs := difference / EpochDuration
	return types.NewBlockHeight(uint64(epochs.Seconds()))
}
