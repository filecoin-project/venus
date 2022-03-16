package node

import (
	"time"

	"github.com/filecoin-project/venus/pkg/clock"
	"github.com/filecoin-project/venus/pkg/journal"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/util/ffiwrapper"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p"
)

// Builder private method accessors for impl's

type builder Builder

// repo returns the repo.
func (b Builder) Repo() repo.Repo {
	return b.repo
}

// GenesisCid read genesis block cid
func (b builder) GenesisCid() cid.Cid {
	return b.genBlk.Cid()
}

// GenesisRoot read genesis block root
func (b builder) GenesisRoot() cid.Cid {
	return b.genBlk.ParentStateRoot
}

//todo remove block time
// BlockTime get chain block time
func (b builder) BlockTime() time.Duration {
	return b.blockTime
}

// Repo get home data repo
func (b builder) Repo() repo.Repo {
	return b.repo
}

// IsRelay get whether the p2p network support replay
func (b builder) IsRelay() bool {
	return b.isRelay
}

// ChainClock get chain clock
func (b builder) ChainClock() clock.ChainEpochClock {
	return b.chainClock
}

// Journal get journal to record event
func (b builder) Journal() journal.Journal {
	return b.journal
}

// Libp2pOpts get libp2p option
func (b builder) Libp2pOpts() []libp2p.Option {
	return b.libp2pOpts
}

// OfflineMode get the p2p network mode
func (b builder) OfflineMode() bool {
	return b.offlineMode
}

// Verify export ffi verify
func (b builder) Verifier() ffiwrapper.Verifier {
	return b.verifier
}
