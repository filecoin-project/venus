package core

import (
	"context"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/types"
)

// CollectBlocksToCommonAncestor traverses chains from two tipsets (called old and new) until their common
// ancestor, collecting all blocks that are in one chain but not the other.
// The resulting lists of blocks are ordered by decreasing height; the ordering of blocks with the same
// height is undefined until https://github.com/filecoin-project/go-filecoin/issues/2310 is resolved.
func CollectBlocksToCommonAncestor(ctx context.Context, store chain.BlockProvider, oldHead, newHead types.TipSet) (oldBlocks, newBlocks []*types.Block, err error) {
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
	oldTipsets, err := chain.CollectTipSetsOfHeightAtLeast(ctx, oldIter, types.NewBlockHeight(commonHeight+uint64(1)))
	if err != nil {
		return
	}
	for _, ts := range oldTipsets {
		oldBlocks = append(oldBlocks, ts.ToSlice()...)
	}
	newTipsets, err := chain.CollectTipSetsOfHeightAtLeast(ctx, newIter, types.NewBlockHeight(commonHeight+uint64(1)))
	for _, ts := range newTipsets {
		newBlocks = append(newBlocks, ts.ToSlice()...)
	}
	if err != nil {
		return
	}
	return
}
