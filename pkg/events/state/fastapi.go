package state

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/venus-shared/types"
)

type FastChainAPI interface {
	ChainAPI
	ChainGetTipSet(context.Context, types.TipSetKey) (*types.TipSet, error)
}

type fastAPI struct {
	FastChainAPI
}

func WrapFastAPI(api FastChainAPI) ChainAPI {
	return &fastAPI{
		api,
	}
}

func (a *fastAPI) StateGetActor(ctx context.Context, actor address.Address, tsk types.TipSetKey) (*types.Actor, error) {
	ts, err := a.FastChainAPI.ChainGetTipSet(ctx, tsk)
	if err != nil {
		return nil, err
	}
	return a.FastChainAPI.StateGetActor(ctx, actor, ts.Parents())
}
