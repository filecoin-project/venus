package chain

import (
	"context"
	"errors"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/internal/pkg/block"
)

// TipSetProvider provides tipsets for traversal.
type TipSetProvider interface {
	GetTipSet(tsKey block.TipSetKey) (*block.TipSet, error)
}

// IterAncestors returns an iterator over tipset ancestors, yielding first the start tipset and
// then its parent tipsets until (and including) the genesis tipset.
func IterAncestors(ctx context.Context, store TipSetProvider, start *block.TipSet) *TipsetIterator {
	return &TipsetIterator{ctx, store, start}
}

// TipsetIterator is an iterator over tipsets.
type TipsetIterator struct {
	ctx   context.Context
	store TipSetProvider
	value *block.TipSet
}

// Value returns the iterator's current value, if not Complete().
func (it *TipsetIterator) Value() *block.TipSet {
	return it.value
}

// Complete tests whether the iterator is exhausted.
func (it *TipsetIterator) Complete() bool {
	return !it.value.Defined()
}

// Next advances the iterator to the next value.
func (it *TipsetIterator) Next() error {
	select {
	case <-it.ctx.Done():
		return it.ctx.Err()
	default:
		if it.value.EnsureHeight() == 0 {
			it.value = &block.TipSet{}
		} else {
			parentKey, err := it.value.Parents()
			if err == nil {
				it.value, err = it.store.GetTipSet(parentKey)
				return err
			}
			return err
		}
		return nil
	}
}

// BlockProvider provides blocks.
type BlockProvider interface {
	GetBlock(ctx context.Context, cid cid.Cid) (*block.Block, error)
}

// LoadTipSetBlocks loads all the blocks for a tipset from the store.
func LoadTipSetBlocks(ctx context.Context, store BlockProvider, key block.TipSetKey) (*block.TipSet, error) {
	var blocks []*block.Block
	for it := key.Iter(); !it.Complete(); it.Next() {
		blk, err := store.GetBlock(ctx, it.Value())
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, blk)
	}
	return block.NewTipSet(blocks...)
}

type tipsetFromBlockProvider struct {
	ctx    context.Context // Context to use when loading blocks
	blocks BlockProvider   // Provides blocks
}

// TipSetProviderFromBlocks builds a tipset provider backed by a block provider.
// Blocks will be loaded with the provided context, since GetTipSet does not accept a
// context parameter. This can and should be removed when GetTipSet does take a context.
func TipSetProviderFromBlocks(ctx context.Context, blocks BlockProvider) TipSetProvider {
	return &tipsetFromBlockProvider{ctx, blocks}
}

// GetTipSet loads the blocks for a tipset.
func (p *tipsetFromBlockProvider) GetTipSet(tsKey block.TipSetKey) (*block.TipSet, error) {
	return LoadTipSetBlocks(p.ctx, p.blocks, tsKey)
}

// CollectTipsToCommonAncestor traverses chains from two tipsets (called old and new) until their common
// ancestor, collecting all tipsets that are in one chain but not the other.
// The resulting lists of tipsets are ordered by decreasing height.
func CollectTipsToCommonAncestor(ctx context.Context, store TipSetProvider, oldHead, newHead *block.TipSet) (oldTips, newTips []*block.TipSet, err error) {
	oldIter := IterAncestors(ctx, store, oldHead)
	newIter := IterAncestors(ctx, store, newHead)

	commonAncestor, err := FindCommonAncestor(oldIter, newIter)
	if err != nil {
		return
	}
	commonHeight, err := commonAncestor.Height()
	if err != nil {
		return
	}

	// Refresh iterators modified by FindCommonAncestors
	oldIter = IterAncestors(ctx, store, oldHead)
	newIter = IterAncestors(ctx, store, newHead)

	// Add 1 to the height argument so that the common ancestor is not
	// included in the outputs.
	oldTips, err = CollectTipSetsOfHeightAtLeast(ctx, oldIter, commonHeight+1)
	if err != nil {
		return
	}
	newTips, err = CollectTipSetsOfHeightAtLeast(ctx, newIter, commonHeight+1)
	return
}

// ErrNoCommonAncestor is returned when two chains assumed to have a common ancestor do not.
var ErrNoCommonAncestor = errors.New("no common ancestor")

// FindCommonAncestor returns the common ancestor of the two tipsets pointed to
// by the input iterators.  If they share no common ancestor ErrNoCommonAncestor
// will be returned.
func FindCommonAncestor(leftIter, rightIter *TipsetIterator) (*block.TipSet, error) {
	for !rightIter.Complete() && !leftIter.Complete() {
		left := leftIter.Value()
		right := rightIter.Value()

		leftHeight, err := left.Height()
		if err != nil {
			return nil, err
		}
		rightHeight, err := right.Height()
		if err != nil {
			return nil, err
		}

		// Found common ancestor.
		if left.Equals(right) {
			return left, nil
		}

		// Update the pointers.  Pointers move back one tipset if they
		// point to a tipset at the same height or higher than the
		// other pointer's tipset.
		if rightHeight >= leftHeight {
			if err := rightIter.Next(); err != nil {
				return nil, err
			}
		}

		if leftHeight >= rightHeight {
			if err := leftIter.Next(); err != nil {
				return nil, err
			}
		}
	}
	return nil, ErrNoCommonAncestor
}

// CollectTipSetsOfHeightAtLeast collects all tipsets with a height greater
// than or equal to minHeight from the input tipset.
func CollectTipSetsOfHeightAtLeast(ctx context.Context, iterator *TipsetIterator, minHeight abi.ChainEpoch) ([]*block.TipSet, error) {
	var ret []*block.TipSet
	var err error
	var h abi.ChainEpoch
	for ; !iterator.Complete(); err = iterator.Next() {
		if err != nil {
			return nil, err
		}
		h, err = iterator.Value().Height()
		if err != nil {
			return nil, err
		}
		if h < minHeight {
			return ret, nil
		}
		ret = append(ret, iterator.Value())
	}
	return ret, nil
}

// FindTipSetAtEpoch finds the highest tipset with height <= the input epoch
// by traversing backwards from start
func FindTipsetAtEpoch(ctx context.Context, start *block.TipSet, epoch abi.ChainEpoch, reader TipSetProvider) (ts *block.TipSet, err error) {
	iterator := IterAncestors(ctx, reader, start)
	var h abi.ChainEpoch
	searchHeight := epoch
	if searchHeight < 0 {
		searchHeight = 0
	}

	for ; !iterator.Complete(); err = iterator.Next() {
		if err != nil {
			return
		}
		ts = iterator.Value()
		h, err = ts.Height()
		if err != nil {
			return
		}
		if h <= searchHeight {
			break
		}
	}
	// If the iterator completed, ts is the genesis tipset.
	return
}

// FindLatestDRAND returns the latest DRAND entry in the chain beginning at start
func FindLatestDRAND(ctx context.Context, start *block.TipSet, reader TipSetProvider) (*block.BeaconEntry, error) {
	iterator := IterAncestors(ctx, reader, start)
	var err error
	for ; !iterator.Complete(); err = iterator.Next() {
		if err != nil {
			return nil, err
		}
		ts := iterator.Value()
		// DRAND entries must be the same for all blocks on the tipset as
		// an invariant of the tipset provider

		entries := ts.At(0).BeaconEntries
		if len(entries) > 0 {
			return entries[len(entries)-1], nil
		}
		// No entries, simply move on to the next ancestor
	}
	return nil, errors.New("no DRAND entries in chain")
}
