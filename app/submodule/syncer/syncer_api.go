package syncer

import (
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/chainsync/status"
)

type SyncerAPI struct { //nolint
	syncer *SyncerSubmodule
}

// SyncerStatus returns the current status of the active or last active chain sync operation.
func (syncerAPI *SyncerAPI) SyncerStatus() status.Status {
	return syncerAPI.syncer.SyncProvider.Status()
}

// ChainSyncHandleNewTipSet submits a chain head to the syncer for processing.
func (syncerAPI *SyncerAPI) ChainSyncHandleNewTipSet(ci *block.ChainInfo) error {
	return syncerAPI.syncer.SyncProvider.HandleNewTipSet(ci)
}
