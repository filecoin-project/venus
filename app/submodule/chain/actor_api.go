package chain

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/vm"
	xerrors "github.com/pkg/errors"
)

type IActor interface {
	StateGetActor(ctx context.Context, actor address.Address, tsk block.TipSetKey) (*types.Actor, error)
	ActorGetSignature(ctx context.Context, actorAddr address.Address, method abi.MethodNum) (vm.ActorMethodSignature, error)
	ListActor(ctx context.Context) (map[address.Address]*types.Actor, error)
}
type ActorAPI struct {
	chain *ChainSubmodule
}

func NewActorAPI(chain *ChainSubmodule) ActorAPI {
	return ActorAPI{chain: chain}
}

func (actorAPI *ActorAPI) StateGetActor(ctx context.Context, actor address.Address, tsk block.TipSetKey) (*types.Actor, error) {
	ts, err := actorAPI.chain.State.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}

	view, err := actorAPI.chain.State.StateView(ts)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}
	return view.LoadActor(ctx, actor)
}

// ActorGetSignature returns the signature of the given actor's given method.
// The function signature is typically used to enable a caller to decode the
// output of an actor method call (message).
func (actorAPI *ActorAPI) ActorGetSignature(ctx context.Context, actorAddr address.Address, method abi.MethodNum) (vm.ActorMethodSignature, error) {
	return actorAPI.chain.State.GetActorSignature(ctx, actorAddr, method)
}

// ActorLs returns a channel with actors from the latest state on the chain
func (actorAPI *ActorAPI) ListActor(ctx context.Context) (map[address.Address]*types.Actor, error) {
	return actorAPI.chain.State.LsActors(ctx)
}
