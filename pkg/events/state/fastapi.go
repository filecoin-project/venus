package state

import (
	"context"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/types"

	"github.com/filecoin-project/go-address"
)

type FastChainAPI interface {
	ChainAPI
	ChainGetTipSet(block.TipSetKey) (*block.TipSet, error)
}

type fastAPI struct {
	FastChainAPI
}

func WrapFastAPI(api FastChainAPI) ChainAPI {
	return &fastAPI{
		api,
	}
}

func (a *fastAPI) StateGetActor(ctx context.Context, actor address.Address, tsk block.TipSetKey) (*types.Actor, error) {
	ts, err := a.FastChainAPI.ChainGetTipSet( tsk)
	if err != nil {
		return nil, err
	}

	return a.FastChainAPI.StateGetActor(ctx, actor, ts.EnsureParents())
}
