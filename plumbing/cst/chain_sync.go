package cst

import (
	"context"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/types"
)

type chainSync interface {
	HandleNewTipSet(context.Context, *types.ChainInfo, bool) error
	Status() *chain.SyncerStatus
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

// Status returns the syncers current status, this includes whether or not the syncer is currently
// running, the chain being synced, and the time it started processing said chain.
func (chs *ChainSyncProvider) Status() *chain.SyncerStatus {
	return chs.sync.Status()
}

// HandleNewTipSet extends the Syncer's chain store with the given tipset if they
// represent a valid extension. It limits the length of new chains it will
// attempt to validate and caches invalid blocks it has encountered to
// help prevent DOS.
func (chs *ChainSyncProvider) HandleNewTipSet(ctx context.Context, ci *types.ChainInfo, trusted bool) error {
	return chs.sync.HandleNewTipSet(ctx, ci, trusted)
}
