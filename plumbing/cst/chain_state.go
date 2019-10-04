package cst

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/sampling"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

type chainReadWriter interface {
	GetHead() types.TipSetKey
	GetTipSet(types.TipSetKey) (types.TipSet, error)
	GetTipSetState(context.Context, types.TipSetKey) (state.Tree, error)
	SetHead(context.Context, types.TipSet) error
}

// ChainStateReadWriter composes a:
// ChainReader providing read access to the chain and its associated state.
// ChainWriter providing write access to the chain head.
type ChainStateReadWriter struct {
	readWriter      chainReadWriter
	cst             *hamt.CborIpldStore // Provides chain blocks and state trees.
	messageProvider chain.MessageProvider
	actors          builtin.Actors
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

// NewChainStateReadWriter returns a new ChainStateReadWriter.
func NewChainStateReadWriter(crw chainReadWriter, messages chain.MessageProvider, cst *hamt.CborIpldStore, ba builtin.Actors) *ChainStateReadWriter {
	return &ChainStateReadWriter{
		readWriter:      crw,
		cst:             cst,
		messageProvider: messages,
		actors:          ba,
	}
}

// Head returns the head tipset
func (chn *ChainStateReadWriter) Head() types.TipSetKey {
	return chn.readWriter.GetHead()
}

// GetTipSet returns the tipset at the given key
func (chn *ChainStateReadWriter) GetTipSet(key types.TipSetKey) (types.TipSet, error) {
	return chn.readWriter.GetTipSet(key)
}

// Ls returns an iterator over tipsets from head to genesis.
func (chn *ChainStateReadWriter) Ls(ctx context.Context) (*chain.TipsetIterator, error) {
	ts, err := chn.readWriter.GetTipSet(chn.readWriter.GetHead())
	if err != nil {
		return nil, err
	}
	return chain.IterAncestors(ctx, chn.readWriter, ts), nil
}

// GetBlock gets a block by CID
func (chn *ChainStateReadWriter) GetBlock(ctx context.Context, id cid.Cid) (*types.Block, error) {
	var out types.Block
	err := chn.cst.Get(ctx, id, &out)
	return &out, err
}

// GetMessages gets a message collection by CID.
func (chn *ChainStateReadWriter) GetMessages(ctx context.Context, id cid.Cid) ([]*types.SignedMessage, error) {
	return chn.messageProvider.LoadMessages(ctx, id)
}

// GetReceipts gets a receipt collection by CID.
func (chn *ChainStateReadWriter) GetReceipts(ctx context.Context, id cid.Cid) ([]*types.MessageReceipt, error) {
	return chn.messageProvider.LoadReceipts(ctx, id)
}

// SampleRandomness samples randomness from the chain at the given height.
func (chn *ChainStateReadWriter) SampleRandomness(ctx context.Context, sampleHeight *types.BlockHeight) ([]byte, error) {
	head := chn.readWriter.GetHead()
	headTipSet, err := chn.readWriter.GetTipSet(head)
	if err != nil {
		return nil, err
	}
	tipSetBuffer, err := chain.GetRecentAncestors(ctx, headTipSet, chn.readWriter, sampleHeight)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get recent ancestors")
	}

	return sampling.SampleChainRandomness(sampleHeight, tipSetBuffer)
}

// GetActor returns an actor from the latest state on the chain
func (chn *ChainStateReadWriter) GetActor(ctx context.Context, addr address.Address) (*actor.Actor, error) {
	return chn.GetActorAt(ctx, chn.readWriter.GetHead(), addr)
}

// GetActorAt returns an actor at a specified tipset key.
func (chn *ChainStateReadWriter) GetActorAt(ctx context.Context, tipKey types.TipSetKey, addr address.Address) (*actor.Actor, error) {
	st, err := chn.readWriter.GetTipSetState(ctx, tipKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load latest state")
	}

	actr, err := st.GetActor(ctx, addr)
	if err != nil {
		return nil, errors.Wrapf(err, "no actor at address %s", addr)
	}
	return actr, nil
}

// LsActors returns a channel with actors from the latest state on the chain
func (chn *ChainStateReadWriter) LsActors(ctx context.Context) (<-chan state.GetAllActorsResult, error) {
	st, err := chn.readWriter.GetTipSetState(ctx, chn.readWriter.GetHead())
	if err != nil {
		return nil, err
	}
	return state.GetAllActors(ctx, st), nil
}

// GetActorSignature returns the signature of the given actor's given method.
// The function signature is typically used to enable a caller to decode the
// output of an actor method call (message).
func (chn *ChainStateReadWriter) GetActorSignature(ctx context.Context, actorAddr address.Address, method string) (*exec.FunctionSignature, error) {
	if method == "" {
		return nil, ErrNoMethod
	}

	actor, err := chn.GetActor(ctx, actorAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get actor")
	} else if actor.Empty() {
		return nil, ErrNoActorImpl
	}

	// TODO: use chain height to determine protocol version
	executable, err := chn.actors.GetBuiltinActorCode(actor.Code, 0)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load actor code")
	}

	export, ok := executable.Exports()[method]
	if !ok {
		return nil, fmt.Errorf("missing export: %s", method)
	}

	return export, nil
}

// SetHead sets `key` as the new head of this chain iff it exists in the nodes chain store.
func (chn *ChainStateReadWriter) SetHead(ctx context.Context, key types.TipSetKey) error {
	headTs, err := chn.readWriter.GetTipSet(key)
	if err != nil {
		return err
	}
	return chn.readWriter.SetHead(ctx, headTs)
}
