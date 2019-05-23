package cst

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
	"github.com/ipfs/go-hamt-ipld"
	"github.com/pkg/errors"
)

type chainReader interface {
	BlockHeight() (uint64, error)
	GetBlock(context.Context, cid.Cid) (*types.Block, error)
	GetHead() types.SortedCidSet
	GetTipSet(types.SortedCidSet) (*types.TipSet, error)
	GetTipSetStateRoot(tsKey types.SortedCidSet) (cid.Cid, error)
}

// ChainStateProvider composes a chain and a state store to provide access to
// the state (including actors) derived from a chain.
type ChainStateProvider struct {
	reader chainReader         // Provides chain blocks and tipsets.
	cst    *hamt.CborIpldStore // Provides state trees.
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

// NewChainStateProvider returns a new ChainStateProvider.
func NewChainStateProvider(chainReader chainReader, cst *hamt.CborIpldStore) *ChainStateProvider {
	return &ChainStateProvider{
		reader: chainReader,
		cst:    cst,
	}
}

// Head returns the head tipset
func (chn *ChainStateProvider) Head() (*types.TipSet, error) {
	ts, err := chn.reader.GetTipSet(chn.reader.GetHead())
	if err != nil {
		return nil, err
	}
	return ts, nil
}

// Ls returns an iterator over tipsets from head to genesis.
func (chn *ChainStateProvider) Ls(ctx context.Context) (*chain.TipsetIterator, error) {
	ts, err := chn.reader.GetTipSet(chn.reader.GetHead())
	if err != nil {
		return nil, err
	}
	return chain.IterAncestors(ctx, chn.reader, *ts), nil
}

// GetBlock gets a block by CID
func (chn *ChainStateProvider) GetBlock(ctx context.Context, id cid.Cid) (*types.Block, error) {
	return chn.reader.GetBlock(ctx, id)
}

// SampleRandomness samples randomness from the chain at the given height.
func (chn *ChainStateProvider) SampleRandomness(ctx context.Context, sampleHeight *types.BlockHeight) ([]byte, error) {
	tipSetBuffer, err := chain.GetRecentAncestorsOfHeaviestChain(ctx, chn.reader, sampleHeight)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get recent ancestors")
	}

	return sampling.SampleChainRandomness(sampleHeight, tipSetBuffer)
}

// GetActor returns an actor from the latest state on the chain
func (chn *ChainStateProvider) GetActor(ctx context.Context, addr address.Address) (*actor.Actor, error) {
	return chn.GetActorAt(ctx, chn.reader.GetHead(), addr)
}

// GetActorAt returns an actor at a specified tipset key.
func (chn *ChainStateProvider) GetActorAt(ctx context.Context, tipKey types.SortedCidSet, addr address.Address) (*actor.Actor, error) {
	stateCid, err := chn.reader.GetTipSetStateRoot(tipKey)
	if err != nil {
		return nil, err
	}
	tree, err := state.LoadStateTree(ctx, chn.cst, stateCid, builtin.Actors)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load latest state")
	}

	actr, err := tree.GetActor(ctx, addr)
	if err != nil {
		return nil, errors.Wrapf(err, "no actor at address %s", addr)
	}
	return actr, nil
}

// LsActors returns a channel with actors from the latest state on the chain
func (chn *ChainStateProvider) LsActors(ctx context.Context) (<-chan state.GetAllActorsResult, error) {
	st, err := chain.LatestState(ctx, chn.reader, chn.cst)
	if err != nil {
		return nil, err
	}
	return state.GetAllActors(ctx, st), nil
}

// GetActorSignature returns the signature of the given actor's given method.
// The function signature is typically used to enable a caller to decode the
// output of an actor method call (message).
func (chn *ChainStateProvider) GetActorSignature(ctx context.Context, actorAddr address.Address, method string) (*exec.FunctionSignature, error) {
	if method == "" {
		return nil, ErrNoMethod
	}

	actor, err := chn.GetActor(ctx, actorAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get actor")
	} else if actor.Empty() {
		return nil, ErrNoActorImpl
	}

	st, err := chain.LatestState(ctx, chn.reader, chn.cst)
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
