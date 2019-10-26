package chain

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// TipSetProvider provides tipsets for traversal.
type TipSetProvider interface {
	GetTipSet(tsKey block.TipSetKey) (block.TipSet, error)
}

// IterAncestors returns an iterator over tipset ancestors, yielding first the start tipset and
// then its parent tipsets until (and including) the genesis tipset.
func IterAncestors(ctx context.Context, store TipSetProvider, start block.TipSet) *TipsetIterator {
	return &TipsetIterator{ctx, store, start}
}

// TipsetIterator is an iterator over tipsets.
type TipsetIterator struct {
	ctx   context.Context
	store TipSetProvider
	value block.TipSet
}

// Value returns the iterator's current value, if not Complete().
func (it *TipsetIterator) Value() block.TipSet {
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
		parentKey, err := it.value.Parents()
		// Parents is empty (without error) for the genesis tipset.
		if err != nil || parentKey.Len() == 0 {
			it.value = block.UndefTipSet
		} else {
			it.value, err = it.store.GetTipSet(parentKey)
		}
		return err
	}
}

// BlockProvider provides blocks.
type BlockProvider interface {
	GetBlock(ctx context.Context, cid cid.Cid) (*block.Block, error)
}

// LoadTipSetBlocks loads all the blocks for a tipset from the store.
func LoadTipSetBlocks(ctx context.Context, store BlockProvider, key block.TipSetKey) (block.TipSet, error) {
	var blocks []*block.Block
	for it := key.Iter(); !it.Complete(); it.Next() {
		blk, err := store.GetBlock(ctx, it.Value())
		if err != nil {
			return block.UndefTipSet, err
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
func (p *tipsetFromBlockProvider) GetTipSet(tsKey block.TipSetKey) (block.TipSet, error) {
	return LoadTipSetBlocks(p.ctx, p.blocks, tsKey)
}

// CollectTipsToCommonAncestor traverses chains from two tipsets (called old and new) until their common
// ancestor, collecting all tipsets that are in one chain but not the other.
// The resulting lists of tipsets are ordered by decreasing height.
func CollectTipsToCommonAncestor(ctx context.Context, store TipSetProvider, oldHead, newHead block.TipSet) (oldTips, newTips []block.TipSet, err error) {
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
	oldTips, err = CollectTipSetsOfHeightAtLeast(ctx, oldIter, types.NewBlockHeight(commonHeight+uint64(1)))
	if err != nil {
		return
	}
	newTips, err = CollectTipSetsOfHeightAtLeast(ctx, newIter, types.NewBlockHeight(commonHeight+uint64(1)))
	return
}
