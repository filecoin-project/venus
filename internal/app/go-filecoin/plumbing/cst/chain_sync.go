package cst

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chainsync"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chainsync/status"
)

type chainSync interface {
	BlockProposer() chainsync.BlockProposer
	Status() status.Status
}

// ChainSyncProvider provides access to chain sync operations and their status.
type ChainSyncProvider struct {
	sync chainSync
}

// NewChainSyncProvider returns a new ChainSyncProvider.
func NewChainSyncProvider(chainSyncer chainSync) *ChainSyncProvider {
	return &ChainSyncProvider{
		sync: chainSyncer,
	}
}

// Status returns the chains current status, this includes whether or not the syncer is currently
// running, the chain being synced, and the time it started processing said chain.
func (chs *ChainSyncProvider) Status() status.Status {
	return chs.sync.Status()
}

// HandleNewTipSet extends the Syncer's chain store with the given tipset if they
// represent a valid extension. It limits the length of new chains it will
// attempt to validate and caches invalid blocks it has encountered to
// help prevent DOS.
func (chs *ChainSyncProvider) HandleNewTipSet(ci *block.ChainInfo) error {
	return chs.sync.BlockProposer().SendOwnBlock(ci)
}
