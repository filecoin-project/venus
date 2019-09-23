package consensus

import (
	"context"

	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
)

// Abstracts over a store of blockchain state.
type chainStateChainReader interface {
	GetHead() types.TipSetKey
	GetTipSetState(context.Context, types.TipSetKey) (state.Tree, error)
	GetTipSet(types.TipSetKey) (types.TipSet, error)
}

// ActorState knows how to send read-only messages for querying actor state.
type ActorState struct {
	// To get the head tipset state root.
	chainReader chainStateChainReader
	// To load the tree for the head tipset state root.
	cst *hamt.CborIpldStore
	// For vm storage.
	bs bstore.Blockstore
}

// NewActorState constructs a ActorState.
func NewActorState(chainReader chainStateChainReader, cst *hamt.CborIpldStore, bs bstore.Blockstore) *ActorState {
	return &ActorState{chainReader, cst, bs}
}

// ActorStateQueryer permits queries to chain state at a particular tip set.
type ActorStateQueryer interface {
	Query(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error)
}

// Queryer returns a query interface to query the chain at a particular tipset
func (cs ActorState) Queryer(ctx context.Context, baseKey types.TipSetKey) (ActorStateQueryer, error) {
	st, err := cs.chainReader.GetTipSetState(ctx, baseKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load tree for the state root of tipset: %s", baseKey.String())
	}
	base, err := cs.chainReader.GetTipSet(baseKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get tipset: %s", baseKey.String())
	}
	h, err := base.Height()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the head tipset height")
	}

	return cs.StateTreeQueryer(st, types.NewBlockHeight(h)), nil
}

// StateTreeQueryer returns a query interface to query the chain for a particular state tree and optional block height
func (cs ActorState) StateTreeQueryer(st state.Tree, bh *types.BlockHeight) ActorStateQueryer {
	return NewProcessorQueryer(st, vm.NewStorageMap(cs.bs), bh)
}

// ProcessorQueryer queries the chain at a particular tipset
type ProcessorQueryer struct {
	st     state.Tree
	vms    vm.StorageMap
	height *types.BlockHeight
}

// NewProcessorQueryer creates an ActorStateQueryer
func NewProcessorQueryer(st state.Tree, vms vm.StorageMap, height *types.BlockHeight) ActorStateQueryer {
	return &ProcessorQueryer{
		st:     st,
		vms:    vms,
		height: height,
	}
}

// Query sends a read-only message against the state of the provided base tipset.
func (q *ProcessorQueryer) Query(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {
	encodedParams, err := abi.ToEncodedValues(params...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode message params")
	}

	r, ec, err := CallQueryMethod(ctx, q.st, q.vms, to, method, encodedParams, optFrom, q.height)
	if err != nil {
		return nil, errors.Wrap(err, "query method returned an error")
	} else if ec != 0 {
		return nil, errors.Errorf("query method returned a non-zero error code %d", ec)
	}
	return r, nil
}
