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
	"github.com/ipfs/go-cid"
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
	st, err := actr.getLatestState(ctx)
	if err != nil {
		return nil, err
	}
	return st.GetActor(ctx, addr)
}

// Ls returns a slice of actors from the latest state on the chain
func (actr *Actr) Ls(ctx context.Context) (<-chan state.GetAllActorsResult, error) {
	st, err := actr.getLatestState(ctx)
	if err != nil {
		return nil, err
	}
	return state.GetAllActors(ctx, st), nil
}

// GetSignature returns the signature of the given actor's given method.
// The function signature is typically used to enable a caller to decode the
// output of an actor method call (message).
func (actr *Actr) GetSignature(ctx context.Context, actorAddr address.Address, method string) (*exec.FunctionSignature, error) {
	if method == "" {
		return nil, errors.New("no method")
	}

	actor, err := actr.Get(ctx, actorAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get actor")
	} else if actor.Empty() {
		return nil, errors.New("no actor implementation")
	}

	executable, err := actr.getExecutable(ctx, actor.Code)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load actor code")
	}

	export, ok := executable.Exports()[method]
	if !ok {
		return nil, fmt.Errorf("missing export: %s", method)
	}

	return export, nil
}

// getExecutable returns the builtin actor code from the latest state on the chain
func (actr *Actr) getExecutable(ctx context.Context, code cid.Cid) (exec.ExecutableActor, error) {
	st, err := actr.getLatestState(ctx)
	if err != nil {
		return nil, err
	}
	return st.GetBuiltinActorCode(code)
}

// getExecutable returns the builtin actor code from the latest state on the chain
func (actr *Actr) getLatestState(ctx context.Context) (state.Tree, error) {
	head := actr.chain.GetHead()
	tsas, err := actr.chain.GetTipSetAndState(ctx, head)
	if err != nil {
		return nil, err
	}

	return state.LoadStateTree(ctx, actr.cst, tsas.TipSetStateRoot, builtin.Actors)
}
