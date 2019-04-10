package chain

import (
	"context"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/types"
)

// BlockProvider provides blocks. This is a subset of the ReadStore interface.
type BlockProvider interface {
	GetBlock(ctx context.Context, cid cid.Cid) (*types.Block, error)
}

// GetParentTipSet returns the parent tipset of a tipset.
// The result is empty if the tipset has no parents (including if it is empty itself)
func GetParentTipSet(ctx context.Context, store BlockProvider, ts types.TipSet) (types.TipSet, error) {
	newTipSet := types.TipSet{}
	parents, err := ts.Parents()
	if err != nil {
		return nil, err
	}
	for it := parents.Iter(); !it.Complete() && ctx.Err() == nil; it.Next() {
		newBlk, err := store.GetBlock(ctx, it.Value())
		if err != nil {
			return nil, err
		}
		if err := newTipSet.AddBlock(newBlk); err != nil {
			return nil, err
		}
	}
	return newTipSet, nil
}

// IterAncestors returns an iterator over tipset ancestors, yielding first the start tipset and
// then its parent tipsets until (and including) the genesis tipset.
func IterAncestors(ctx context.Context, store BlockProvider, start types.TipSet) *TipsetIterator {
	return &TipsetIterator{ctx, store, start}
}

// TipsetIterator is an iterator over tipsets.
type TipsetIterator struct {
	ctx   context.Context
	store BlockProvider
	value types.TipSet
}

// Value returns the iterator's current value, if not Complete().
func (it *TipsetIterator) Value() types.TipSet {
	return it.value
}

// Complete tests whether the iterator is exhausted.
func (it *TipsetIterator) Complete() bool {
	return len(it.value) == 0
}

// Next advances the iterator to the next value.
func (it *TipsetIterator) Next() error {
	var err error
	select {
	case <-it.ctx.Done():
		return it.ctx.Err()
	default:
		it.value, err = GetParentTipSet(it.ctx, it.store, it.value)
	}
	return err
}
