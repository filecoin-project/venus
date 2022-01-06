package chain

import (
	"context"

	"github.com/filecoin-project/go-address"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var _ v1api.IActor = &actorAPI{}

type actorAPI struct {
	chain *ChainSubmodule
}

//NewActorAPI new actor api
func NewActorAPI(chain *ChainSubmodule) v1api.IActor {
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
