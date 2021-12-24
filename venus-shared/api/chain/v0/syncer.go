package v0

import (
	"context"

	"github.com/filecoin-project/go-state-types/big"

	chain2 "github.com/filecoin-project/venus/venus-shared/api/chain"
	"github.com/filecoin-project/venus/venus-shared/chain"
)

type ISyncer interface {
	ChainSyncHandleNewTipSet(ctx context.Context, ci *chain.ChainInfo) error                             //perm:write
	SetConcurrent(ctx context.Context, concurrent int64) error                                           //perm:admin
	SyncerTracker(ctx context.Context) *chain2.TargetTracker                                             //perm:read
	Concurrent(ctx context.Context) int64                                                                //perm:read
	ChainTipSetWeight(ctx context.Context, tsk chain.TipSetKey) (big.Int, error)                         //perm:read
	SyncSubmitBlock(ctx context.Context, blk *chain.BlockMsg) error                                      //perm:write
	StateCall(ctx context.Context, msg *chain.Message, tsk chain.TipSetKey) (*chain2.InvocResult, error) //perm:read
	SyncState(ctx context.Context) (*chain2.SyncState, error)                                            //perm:read
}
