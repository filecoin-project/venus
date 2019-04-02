package actr

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	hamt "github.com/ipfs/go-hamt-ipld"
	"github.com/pkg/errors"
)

// Abstracts over a store of blockchain state.
type chainReader interface {
	GetHead() types.SortedCidSet
	GetTipSetAndState(ctx context.Context, tsKey types.SortedCidSet) (*chain.TipSetAndState, error)
}

type Actr struct {
	chain chainReader
	cst   *hamt.CborIpldStore
}

func NewActr(chnReader chainReader, cst *hamt.CborIpldStore) *Actr {
	return &Actr{
		chain: chnReader,
		cst:   cst,
	}
}

// Get returns an actor from the latest state on the chain
func (actr *Actr) Get(ctx context.Context, addr address.Address) (*actor.Actor, error) {
	head := actr.chain.GetHead()
	tsas, err := actr.chain.GetTipSetAndState(ctx, head)
	if err != nil {
		return nil, err
	}

	state, err := state.LoadStateTree(ctx, actr.cst, tsas.TipSetStateRoot, builtin.Actors)
	if err != nil {
		return nil, err
	}
	return state.GetActor(ctx, addr)
}

// Ls returns a slice of actors from the latest state on the chain
func (actr *Actr) Ls(ctx context.Context) (<-chan state.GetAllActorsResult, error) {
	head := actr.chain.GetHead()
	tsas, err := actr.chain.GetTipSetAndState(ctx, head)
	if err != nil {
		return nil, err
	}

	st, err := state.LoadStateTree(ctx, actr.cst, tsas.TipSetStateRoot, builtin.Actors)
	if err != nil {
		return nil, err
	}
	return state.GetAllActors(ctx, st), nil
}

// GetSignature returns the signature of the given actor's given method.
// The function signature is typically used to enable a caller to decode the
// output of an actor method call (message).
func (actr *Actr) GetSignature(ctx context.Context, actorAddr address.Address, method string) (_ *exec.FunctionSignature, err error) {
	head := actr.chain.GetHead()
	tsas, err := actr.chain.GetTipSetAndState(ctx, head)
	if err != nil {
		return nil, err
	}

	st, err := state.LoadStateTree(ctx, actr.cst, tsas.TipSetStateRoot, builtin.Actors)
	if err != nil {
		return nil, errors.Wrap(err, "couldnt get current state tree")
	}

	actor, err := st.GetActor(ctx, actorAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get actor")
	} else if actor.Empty() {
		return nil, errors.New("no actor implementation")
	}

	executable, err := st.GetBuiltinActorCode(actor.Code)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load actor code")
	}

	if method == "" {
		return nil, errors.New("no method")
	}

	export, ok := executable.Exports()[method]
	if !ok {
		return nil, fmt.Errorf("missing export: %s", method)
	}

	return export, nil
}
