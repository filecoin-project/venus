package v0

import (
	"context"

	"github.com/filecoin-project/go-state-types/big"

	"github.com/filecoin-project/venus/venus-shared/types"
)

type ISyncer interface {
	ChainSyncHandleNewTipSet(ctx context.Context, ci *types.ChainInfo) error     //perm:write
	SetConcurrent(ctx context.Context, concurrent int64) error                   //perm:admin
	SyncerTracker(ctx context.Context) *types.TargetTracker                      //perm:read
	Concurrent(ctx context.Context) int64                                        //perm:read
	ChainTipSetWeight(ctx context.Context, tsk types.TipSetKey) (big.Int, error) //perm:read
	SyncSubmitBlock(ctx context.Context, blk *types.BlockMsg) error              //perm:write
	SyncState(ctx context.Context) (*types.SyncState, error)                     //perm:read
	// SyncIncomingBlocks returns a channel streaming incoming, potentially not
	// yet synced block headers.
	SyncIncomingBlocks(ctx context.Context) (<-chan *types.BlockHeader, error) //perm:read
	// SyncCheckpoint marks a blocks as checkpointed, meaning that it won't ever fork away from it.
	SyncCheckpoint(ctx context.Context, tsk types.TipSetKey) error //perm:admin
}
