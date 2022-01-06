package v0api

import (
	"context"

	"github.com/filecoin-project/go-state-types/big"
	syncTypes "github.com/filecoin-project/venus/pkg/chainsync/types"
	"github.com/filecoin-project/venus/venus-shared/types"
)

type ISyncer interface {
	// Rule[perm:write]
	ChainSyncHandleNewTipSet(ctx context.Context, ci *types.ChainInfo) error
	// Rule[perm:admin]
	SetConcurrent(ctx context.Context, concurrent int64) error
	// Rule[perm:read]
	SyncerTracker(ctx context.Context) *syncTypes.TargetTracker
	// Rule[perm:read]
	Concurrent(ctx context.Context) int64
	// Rule[perm:read]
	ChainTipSetWeight(ctx context.Context, tsk types.TipSetKey) (big.Int, error)
	// Rule[perm:write]
	SyncSubmitBlock(ctx context.Context, blk *types.BlockMsg) error
	// Rule[perm:read]
	StateCall(ctx context.Context, msg *types.Message, tsk types.TipSetKey) (*types.InvocResult, error)
	// Rule[perm:read]
	SyncState(ctx context.Context) (*types.SyncState, error)
}
