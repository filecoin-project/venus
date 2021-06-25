package chain

import (
	"context"
	"github.com/filecoin-project/venus/app/submodule/apiface"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/pkg/types"
	xerrors "github.com/pkg/errors"
)

var _ apiface.IActor = &actorAPI{}

type actorAPI struct {
	chain *ChainSubmodule
}

//NewActorAPI new actor api
func NewActorAPI(chain *ChainSubmodule) apiface.IActor {
	return &actorAPI{chain: chain}
}

// StateGetActor returns the indicated actor's nonce and balance.
func (actorAPI *actorAPI) StateGetActor(ctx context.Context, actor address.Address, tsk types.TipSetKey) (*types.Actor, error) {
	ts, err := actorAPI.chain.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}

	view, err := actorAPI.chain.ChainReader.ParentStateView(ts)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}
	return view.LoadActor(ctx, actor)
}

// ActorLs returns a channel with actors from the latest state on the chain
func (actorAPI *actorAPI) ListActor(ctx context.Context) (map[address.Address]*types.Actor, error) {
	return actorAPI.chain.ChainReader.LsActors(ctx)
}
