package v0

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	chain2 "github.com/filecoin-project/venus/venus-shared/api/chain"
	"github.com/filecoin-project/venus/venus-shared/chain"
)

type IMining interface {
	// Rule[perm:read]
	MinerGetBaseInfo(ctx context.Context, maddr address.Address, round abi.ChainEpoch, tsk chain.TipSetKey) (*chain2.MiningBaseInfo, error)
	// Rule[perm:write]
	MinerCreateBlock(ctx context.Context, bt *chain2.BlockTemplate) (*chain.BlockMsg, error)
}
