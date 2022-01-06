package v0

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/venus/venus-shared/types"
)

type IMining interface {
	MinerGetBaseInfo(ctx context.Context, maddr address.Address, round abi.ChainEpoch, tsk types.TipSetKey) (*types.MiningBaseInfo, error) //perm:read
	MinerCreateBlock(ctx context.Context, bt *types.BlockTemplate) (*types.BlockMsg, error)                                                //perm:write
}
