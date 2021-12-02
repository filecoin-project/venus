package v1

import (
	"context"

	"github.com/filecoin-project/go-state-types/big"

	"github.com/filecoin-project/venus/venus-shared/chain"
)

type ISyncer interface {
	// Rule[perm:read]
	ChainSyncHandleNewTipSet(ctx context.Context, ci *ChainInfo) error
	// Rule[perm:read]
	SetConcurrent(ctx context.Context, concurrent int64) error

	// Rule[perm:read]
	// SyncerTracker(ctx context.Context) *syncTypes.TargetTracker

	// Rule[perm:read]
	Concurrent(ctx context.Context) int64
	// Rule[perm:read]
	ChainTipSetWeight(ctx context.Context, tsk chain.TipSetKey) (big.Int, error)
	// Rule[perm:read]
	SyncSubmitBlock(ctx context.Context, blk *chain.BlockMsg) error
	// Rule[perm:read]
	StateCall(ctx context.Context, msg *chain.Message, tsk chain.TipSetKey) (*InvocResult, error)
	// Rule[perm:read]
	SyncState(ctx context.Context) (*SyncState, error)
}
