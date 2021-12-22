package v0api

import (
	"context"

	"github.com/filecoin-project/go-state-types/big"

	chain2 "github.com/filecoin-project/venus/venus-shared/api/chain"
	"github.com/filecoin-project/venus/venus-shared/chain"
)

type ISyncer interface {
	// Rule[perm:write]
	ChainSyncHandleNewTipSet(ctx context.Context, ci *chain.ChainInfo) error
	// Rule[perm:admin]
	SetConcurrent(ctx context.Context, concurrent int64) error

	// Rule[perm:read]
	//SyncerTracker(ctx context.Context) *chain.TargetTracker

	// Rule[perm:read]
	Concurrent(ctx context.Context) int64
	// Rule[perm:read]
	ChainTipSetWeight(ctx context.Context, tsk chain.TipSetKey) (big.Int, error)
	// Rule[perm:write]
	SyncSubmitBlock(ctx context.Context, blk *chain.BlockMsg) error
	// Rule[perm:read]
	StateCall(ctx context.Context, msg *chain.Message, tsk chain.TipSetKey) (*chain2.InvocResult, error)
	// Rule[perm:read]
	SyncState(ctx context.Context) (*chain2.SyncState, error)
}
