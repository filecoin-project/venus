package actr

import (
	"context"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/plumbing/mthdsig"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"

	hamt "gx/ipfs/QmNf3wujpV2Y7Lnj2hy2UrmuX8bhMDStRHbnSLh7Ypf36h/go-hamt-ipld"
)

// Abstracts over a store of blockchain state.
type chainReader interface {
	GetHead() types.SortedCidSet
	GetTipSetAndState(ctx context.Context, tsKey types.SortedCidSet) (*chain.TipSetAndState, error)
}

type Actr struct {
	chain     chainReader
	cst       *hamt.CborIpldStore
	sigGetter *mthdsig.Getter
}

func NewActr(chnReader chainReader, cst *hamt.CborIpldStore, sigGetter *mthdsig.Getter) *Actr {
	return &Actr{
		chain:     chnReader,
		cst:       cst,
		sigGetter: sigGetter,
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
	return actr.sigGetter.Get(ctx, actorAddr, method)
}
