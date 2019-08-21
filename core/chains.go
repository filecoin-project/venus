package core

import (
	"context"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/types"
)

// CollectTipsToCommonAncestor traverses chains from two tipsets (called old and new) until their common
// ancestor, collecting all tipsets that are in one chain but not the other.
// The resulting lists of tipsets are ordered by decreasing height.
func CollectTipsToCommonAncestor(ctx context.Context, store chain.TipSetProvider, oldHead, newHead types.TipSet) (oldTips, newTips []types.TipSet, err error) {
	oldIter := chain.IterAncestors(ctx, store, oldHead)
	newIter := chain.IterAncestors(ctx, store, newHead)

	commonAncestor, err := chain.FindCommonAncestor(oldIter, newIter)
	if err != nil {
		return
	}
	commonHeight, err := commonAncestor.Height()
	if err != nil {
		return
	}

	// Refresh iterators modified by FindCommonAncestors
	oldIter = chain.IterAncestors(ctx, store, oldHead)
	newIter = chain.IterAncestors(ctx, store, newHead)

	// Add 1 to the height argument so that the common ancestor is not
	// included in the outputs.
	oldTips, err = CollectTipSetsOfHeight(ctx, oldIter, types.NewBlockHeight(commonHeight+uint64(1)))
	if err != nil {
		return
	}
	newTips, err = CollectTipSetsOfHeight(ctx, newIter, types.NewBlockHeight(commonHeight+uint64(1)))
	return
}

// CollectTipSetsOfHeight collects all tipsets with a height greater than the earliest
// possible proving period start still in scope for the given head.
func CollectTipSetsOfHeight(ctx context.Context, iterator *chain.TipsetIterator, minHeight *types.BlockHeight) ([]types.TipSet, error) {
	var ret []types.TipSet
	var err error
	var h uint64
	for ; !iterator.Complete(); err = iterator.Next() {
		if err != nil {
			return nil, err
		}
		h, err = iterator.Value().Height()
		if err != nil {
			return nil, err
		}
		if types.NewBlockHeight(h).LessThan(minHeight) {
			break
		}
		ret = append(ret, iterator.Value())
	}

	return ret, nil
}
