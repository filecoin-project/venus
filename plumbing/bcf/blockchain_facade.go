package bcf

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/sampling"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/ipfs/go-cid"
	hamt "github.com/ipfs/go-hamt-ipld"
	"github.com/pkg/errors"
)

// BlockChainFacade is a facade pattern for the chain core api. It provides a
// simple, unified interface to the complex set of calls in chain, state, exec,
// and sampling for use in protocols and commands.
type BlockChainFacade struct {
	// To get the head tipset state root.
	reader chain.ReadStore
	// To load the tree for the head tipset state root.
	cst *hamt.CborIpldStore
}

var (
	// ErrNoMethod is returned by Get when there is no method signature (eg, transfer).
	ErrNoMethod = errors.New("no method")
	// ErrNoActorImpl is returned by Get when the actor implementation doesn't exist, eg
	// the actor address is an empty actor, an address that has received a transfer of FIL
	// but hasn't yet been upgraded to an account actor. (The actor implementation might
	// also genuinely be missing, which is not expected.)
	ErrNoActorImpl = errors.New("no actor implementation")
)

// NewBlockChainFacade returns a new BlockChainFacade.
func NewBlockChainFacade(chainReader chain.ReadStore, cst *hamt.CborIpldStore) *BlockChainFacade {
	return &BlockChainFacade{
		reader: chainReader,
		cst:    cst,
	}
}

// Head returns the head tipset
func (chn *BlockChainFacade) Head() (*types.TipSet, error) {
	ts, err := chn.reader.GetTipSet(chn.reader.GetHead())
	if err != nil {
		return nil, err
	}
	return ts, nil
}

// Ls returns a channel of tipsets from head to genesis
func (chn *BlockChainFacade) Ls(ctx context.Context) (*chain.TipsetIterator, error) {
	ts, err := chn.reader.GetTipSet(chn.reader.GetHead())
	if err != nil {
		return nil, err
	}
	return chain.IterAncestors(ctx, chn.reader, *ts), nil
}

// GetBlock gets a block by CID
func (chn *BlockChainFacade) GetBlock(ctx context.Context, id cid.Cid) (*types.Block, error) {
	return chn.reader.GetBlock(ctx, id)
}

// SampleRandomness samples randomness from the chain at the given height.
func (chn *BlockChainFacade) SampleRandomness(ctx context.Context, sampleHeight *types.BlockHeight) ([]byte, error) {
	tipSetBuffer, err := chain.GetRecentAncestorsOfHeaviestChain(ctx, chn.reader, sampleHeight)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get recent ancestors")
	}

	return sampling.SampleChainRandomness(sampleHeight, tipSetBuffer)
}

// GetActor returns an actor from the latest state on the chain
func (chn *BlockChainFacade) GetActor(ctx context.Context, addr address.Address) (*actor.Actor, error) {
	st, err := chn.getLatestState(ctx)
	if err != nil {
		return nil, err
	}
	return st.GetActor(ctx, addr)
}

// LsActors returns a channel with actors from the latest state on the chain
func (chn *BlockChainFacade) LsActors(ctx context.Context) (<-chan state.GetAllActorsResult, error) {
	st, err := chn.getLatestState(ctx)
	if err != nil {
		return nil, err
	}
	return state.GetAllActors(ctx, st), nil
}

// GetActorSignature returns the signature of the given actor's given method.
// The function signature is typically used to enable a caller to decode the
// output of an actor method call (message).
func (chn *BlockChainFacade) GetActorSignature(ctx context.Context, actorAddr address.Address, method string) (*exec.FunctionSignature, error) {
	if method == "" {
		return nil, ErrNoMethod
	}

	actor, err := chn.GetActor(ctx, actorAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get actor")
	} else if actor.Empty() {
		return nil, ErrNoActorImpl
	}

	st, err := chn.getLatestState(ctx)
	if err != nil {
		return nil, err
	}

	executable, err := st.GetBuiltinActorCode(actor.Code)
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
func (chn *BlockChainFacade) getLatestState(ctx context.Context) (state.Tree, error) {
	head := chn.reader.GetHead()
	stateCid, err := chn.reader.GetTipSetStateRoot(head)
	if err != nil {
		return nil, err
	}

	return state.LoadStateTree(ctx, chn.cst, stateCid, builtin.Actors)
}
