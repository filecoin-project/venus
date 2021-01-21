package state

import (
	"context"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/types"

	"github.com/filecoin-project/go-address"

)

type FastChainApiAPI interface {
	ChainAPI

	ChainGetTipSet(context.Context, block.TipSetKey) (*block.TipSet, error)
}

type fastAPI struct {
	FastChainApiAPI
}

func WrapFastAPI(api FastChainApiAPI) ChainAPI {
	return &fastAPI{
		api,
	}
}

func (a *fastAPI) StateGetActor(ctx context.Context, actor address.Address, tsk block.TipSetKey) (*types.Actor, error) {
	ts, err := a.FastChainApiAPI.ChainGetTipSet(ctx, tsk)
	if err != nil {
		return nil, err
	}

	return a.FastChainApiAPI.StateGetActor(ctx, actor, ts.Parents())
}
