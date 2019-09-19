package chain

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/processor"
	"github.com/filecoin-project/go-filecoin/vm"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/sampling"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

type chainReader interface {
	GetHead() types.TipSetKey
	GetTipSet(types.TipSetKey) (types.TipSet, error)
	GetTipSetState(context.Context, types.TipSetKey) (state.Tree, error)
}

// StateProvider composes a chain and a state store to provide access to
// the state (including actors) derived from a chain.
type StateProvider struct {
	reader          chainReader           // Provides chain tipsets and state roots.
	cst             *hamt.CborIpldStore   // Provides chain blocks and state trees.
	bs              blockstore.Blockstore // For vm storage.
	messageProvider MessageProvider
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

// NewStateProvider returns a new StateProvider.
func NewStateProvider(chainReader chainReader, messages MessageProvider, cst *hamt.CborIpldStore, bs blockstore.Blockstore) *StateProvider {
	return &StateProvider{
		reader:          chainReader,
		cst:             cst,
		messageProvider: messages,
		bs:              bs,
	}
}

// Head returns the head tipset
func (sp *StateProvider) Head() types.TipSetKey {
	return sp.reader.GetHead()
}

// GetTipSet returns the tipset at the given key
func (sp *StateProvider) GetTipSet(key types.TipSetKey) (types.TipSet, error) {
	return sp.reader.GetTipSet(key)
}

// Ls returns an iterator over tipsets from head to genesis.
func (sp *StateProvider) Ls(ctx context.Context) (*TipsetIterator, error) {
	ts, err := sp.reader.GetTipSet(sp.reader.GetHead())
	if err != nil {
		return nil, err
	}
	return IterAncestors(ctx, sp.reader, ts), nil
}

// GetBlock gets a block by CID
func (sp *StateProvider) GetBlock(ctx context.Context, id cid.Cid) (*types.Block, error) {
	var out types.Block
	err := sp.cst.Get(ctx, id, &out)
	return &out, err
}

// GetMessages gets a message collection by CID.
func (sp *StateProvider) GetMessages(ctx context.Context, id cid.Cid) ([]*types.SignedMessage, error) {
	return sp.messageProvider.LoadMessages(ctx, id)
}

// GetReceipts gets a receipt collection by CID.
func (sp *StateProvider) GetReceipts(ctx context.Context, id cid.Cid) ([]*types.MessageReceipt, error) {
	return sp.messageProvider.LoadReceipts(ctx, id)
}

// SampleRandomness samples randomness from the chain at the given height.
func (sp *StateProvider) SampleRandomness(ctx context.Context, sampleHeight *types.BlockHeight) ([]byte, error) {
	head := sp.reader.GetHead()
	headTipSet, err := sp.reader.GetTipSet(head)
	if err != nil {
		return nil, err
	}
	tipSetBuffer, err := GetRecentAncestors(ctx, headTipSet, sp.reader, sampleHeight)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get recent ancestors")
	}

	return sampling.SampleChainRandomness(sampleHeight, tipSetBuffer)
}

// GetActor returns an actor from the latest state on the chain
func (sp *StateProvider) GetActor(ctx context.Context, addr address.Address) (*actor.Actor, error) {
	return sp.GetActorAt(ctx, sp.reader.GetHead(), addr)
}

// GetActorAt returns an actor at a specified tipset key.
func (sp *StateProvider) GetActorAt(ctx context.Context, tipKey types.TipSetKey, addr address.Address) (*actor.Actor, error) {
	st, err := sp.reader.GetTipSetState(ctx, tipKey)
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
func (sp *StateProvider) LsActors(ctx context.Context) (<-chan state.GetAllActorsResult, error) {
	st, err := sp.reader.GetTipSetState(ctx, sp.reader.GetHead())
	if err != nil {
		return nil, err
	}
	return state.GetAllActors(ctx, st), nil
}

// GetActorSignature returns the signature of the given actor's given method.
// The function signature is typically used to enable a caller to decode the
// output of an actor method call (message).
func (sp *StateProvider) GetActorSignature(ctx context.Context, actorAddr address.Address, method string) (*exec.FunctionSignature, error) {
	if method == "" {
		return nil, ErrNoMethod
	}

	actor, err := sp.GetActor(ctx, actorAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get actor")
	} else if actor.Empty() {
		return nil, ErrNoActorImpl
	}

	st, err := sp.reader.GetTipSetState(ctx, sp.reader.GetHead())
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

// Queryer returns a query interface to query the chain at a particular tipset
func (sp *StateProvider) Queryer(ctx context.Context, baseKey types.TipSetKey) (StateQueryer, error) {
	st, err := sp.reader.GetTipSetState(ctx, baseKey)
	if err != nil {
		return StateQueryer{}, errors.Wrapf(err, "failed to load tree for the state root of tipset: %s", baseKey.String())
	}
	base, err := sp.reader.GetTipSet(baseKey)
	if err != nil {
		return StateQueryer{}, errors.Wrapf(err, "failed to get tipset: %s", baseKey.String())
	}
	h, err := base.Height()
	if err != nil {
		return StateQueryer{}, errors.Wrap(err, "failed to get the head tipset height")
	}

	return StateQueryer{
		st:     st,
		vms:    vm.NewStorageMap(sp.bs),
		height: types.NewBlockHeight(h),
	}, nil
}

// StateQueryer queries the chain at a particular tipset
type StateQueryer struct {
	st     state.Tree
	vms    vm.StorageMap
	height *types.BlockHeight
}

// Query sends a read-only message against the state of the provided base tipset.
func (sq *StateQueryer) Query(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {
	encodedParams, err := abi.ToEncodedValues(params...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode message params")
	}

	r, ec, err := processor.CallQueryMethod(ctx, sq.st, sq.vms, to, method, encodedParams, optFrom, sq.height)
	if err != nil {
		return nil, errors.Wrap(err, "querymethod returned an error")
	} else if ec != 0 {
		return nil, errors.Errorf("querymethod returned a non-zero error code %d", ec)
	}
	return r, nil
}
