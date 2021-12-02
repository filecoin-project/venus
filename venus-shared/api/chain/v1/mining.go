package v1

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/venus/venus-shared/chain"
)

type IMining interface {
	// Rule[perm:read]
	MinerGetBaseInfo(ctx context.Context, maddr address.Address, round abi.ChainEpoch, tsk chain.TipSetKey) (*MiningBaseInfo, error)
	// Rule[perm:read]
	MinerCreateBlock(ctx context.Context, bt *BlockTemplate) (*chain.BlockMsg, error)
}
