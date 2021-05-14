package chain

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/pkg/types"
	xerrors "github.com/pkg/errors"
)

type IActor interface {
	StateGetActor(ctx context.Context, actor address.Address, tsk types.TipSetKey) (*types.Actor, error)
	ListActor(ctx context.Context) (map[address.Address]*types.Actor, error)
}

var _ IActor = &ActorAPI{}

type ActorAPI struct {
	chain *ChainSubmodule
}

func NewActorAPI(chain *ChainSubmodule) ActorAPI {
	return ActorAPI{chain: chain}
}

func (actorAPI *ActorAPI) StateGetActor(ctx context.Context, actor address.Address, tsk types.TipSetKey) (*types.Actor, error) {
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
func (actorAPI *ActorAPI) ListActor(ctx context.Context) (map[address.Address]*types.Actor, error) {
	return actorAPI.chain.ChainReader.LsActors(ctx)
}
