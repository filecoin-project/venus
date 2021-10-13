package syncer

import (
	"github.com/filecoin-project/venus/pkg/chainsync"
	"github.com/filecoin-project/venus/pkg/types"
	"time"
)

type chainSync interface {
	BlockProposer() chainsync.BlockProposer
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

// HandleNewTipSet extends the Syncer's chain store with the given tipset if they
// represent a valid extension. It limits the length of new chains it will
// attempt to validate and caches invalid blocks it has encountered to
// help prevent DOS.
func (chs *ChainSyncProvider) HandleNewTipSet(ci *types.ChainInfo) error {
	return chs.sync.BlockProposer().SendOwnBlock(ci)
}

const incomeBlockLargeDelayDuration = time.Second * 5
const slowFetchMessageDuration = time.Second * 3
