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

func (chainAPI *ChainAPI) StateGetActor(ctx context.Context, actor address.Address, tsk block.TipSetKey) (*types.Actor, error) {
	view, err := chainAPI.chain.State.StateView(tsk)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset %s: %w", tsk, err)
	}
	return view.LoadActor(ctx, actor)
}

// ActorGetSignature returns the signature of the given actor's given method.
// The function signature is typically used to enable a caller to decode the
// output of an actor method call (message).
func (chainAPI *ChainAPI) ActorGetSignature(ctx context.Context, actorAddr address.Address, method abi.MethodNum) (vm.ActorMethodSignature, error) {
	return chainAPI.chain.State.GetActorSignature(ctx, actorAddr, method)
}

// ActorLs returns a channel with actors from the latest state on the chain
func (chainAPI *ChainAPI) ListActor(ctx context.Context) (map[address.Address]*types.Actor, error) {
	return chainAPI.chain.State.LsActors(ctx)
}
