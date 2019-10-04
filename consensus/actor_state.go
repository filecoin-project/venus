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

// ActorStateStore knows how to send read-only messages for querying actor state.
type ActorStateStore struct {
	// To get the head tipset state root.
	chainReader chainStateChainReader
	// To load the tree for the head tipset state root.
	cst *hamt.CborIpldStore
	// For vm storage.
	bs bstore.Blockstore
	// executable actors
	processor Processor
}

// NewActorStateStore constructs a ActorStateStore.
func NewActorStateStore(chainReader chainStateChainReader, cst *hamt.CborIpldStore, bs bstore.Blockstore, processor Processor) *ActorStateStore {
	return &ActorStateStore{chainReader, cst, bs, processor}
}

// ActorStateSnapshot permits queries to chain state at a particular tip set.
type ActorStateSnapshot interface {
	Query(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error)
}

// Snapshot returns a snapshot of tipset state for querying
func (cs ActorStateStore) Snapshot(ctx context.Context, baseKey types.TipSetKey) (ActorStateSnapshot, error) {
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

	return cs.StateTreeSnapshot(st, types.NewBlockHeight(h)), nil
}

// StateTreeSnapshot returns a snapshot representation of a state tree at an optional block height
func (cs ActorStateStore) StateTreeSnapshot(st state.Tree, bh *types.BlockHeight) ActorStateSnapshot {
	return newProcessorQueryer(st, vm.NewStorageMap(cs.bs), bh, cs.processor)
}

// processorSnapshot queries the chain at a particular tipset
type processorSnapshot struct {
	st        state.Tree
	vms       vm.StorageMap
	height    *types.BlockHeight
	processor Processor
}

// newProcessorQueryer creates an ActorStateSnapshot
func newProcessorQueryer(st state.Tree, vms vm.StorageMap, height *types.BlockHeight, processor Processor) ActorStateSnapshot {
	return &processorSnapshot{
		st:        st,
		vms:       vms,
		height:    height,
		processor: processor,
	}
}

// Query sends a read-only message against the state of the snapshot.
func (q *processorSnapshot) Query(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {
	encodedParams, err := abi.ToEncodedValues(params...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode message params")
	}

	r, ec, err := q.processor.CallQueryMethod(ctx, q.st, q.vms, to, method, encodedParams, optFrom, q.height)
	if err != nil {
		return nil, errors.Wrap(err, "query method returned an error")
	} else if ec != 0 {
		return nil, errors.Errorf("query method returned a non-zero error code %d", ec)
	}
	return r, nil
}
