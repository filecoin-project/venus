package consensus

import (
	"context"

	bstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

// Abstracts over a store of blockchain state.
type chainStateChainReader interface {
	GetHead() block.TipSetKey
	GetTipSetState(context.Context, block.TipSetKey) (state.Tree, error)
	GetTipSet(block.TipSetKey) (block.TipSet, error)
}

// QueryProcessor querys actor state of a particular tipset
// Dragons: delete
type QueryProcessor interface {
}

// ActorStateStore knows how to send read-only messages for querying actor state.
type ActorStateStore struct {
	// To get the head tipset state root.
	chainReader chainStateChainReader
	// To load the tree for the head tipset state root.
	cst cbor.IpldStore
	// For vm storage.
	bs bstore.Blockstore
	// executable actors
	processor QueryProcessor
}

// NewActorStateStore constructs a ActorStateStore.
func NewActorStateStore(chainReader chainStateChainReader, cst cbor.IpldStore, bs bstore.Blockstore, processor QueryProcessor) *ActorStateStore {
	return &ActorStateStore{chainReader, cst, bs, processor}
}

// ActorStateSnapshot permits queries to chain state at a particular tip set.
// Dragons: delete
type ActorStateSnapshot interface {
}

// Snapshot returns a snapshot of tipset state for querying
func (cs ActorStateStore) Snapshot(ctx context.Context, baseKey block.TipSetKey) (ActorStateSnapshot, error) {
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
	return newProcessorQueryer(st, bh, cs.processor)
}

// processorSnapshot queries the chain at a particular tipset
type processorSnapshot struct {
	st        state.Tree
	height    *types.BlockHeight
	processor QueryProcessor
}

// newProcessorQueryer creates an ActorStateSnapshot
func newProcessorQueryer(st state.Tree, height *types.BlockHeight, processor QueryProcessor) ActorStateSnapshot {
	return &processorSnapshot{
		st:        st,
		height:    height,
		processor: processor,
	}
}
