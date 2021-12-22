package chain

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/app/client/apiface"
	types "github.com/filecoin-project/venus/venus-shared/chain"
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
	return actorAPI.chain.Stmgr.GetActorAtTsk(ctx, actor, tsk)
}

// ActorLs returns a channel with actors from the latest state on the chain
func (actorAPI *actorAPI) ListActor(ctx context.Context) (map[address.Address]*types.Actor, error) {
	return actorAPI.chain.ChainReader.LsActors(ctx)
}
