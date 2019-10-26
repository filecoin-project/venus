package chain

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/go-filecoin/internal/pkg/metrics/tracing"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// ErrNoCommonAncestor is returned when two chains assumed to have a common ancestor do not.
var ErrNoCommonAncestor = errors.New("no common ancestor")

// GetRecentAncestors returns the ancestors of base as a slice of TipSets down to and including the
// first non-empty tipset with height <= `minHeight` (or the genesis tipset if minHeight is negative).
//
// Because null blocks increase chain height but do not have associated tipsets
// the length of the returned list may vary (more null blocks -> shorter length).
// This is all more complex than necessary, we should just index tipsets by height:
// https://github.com/filecoin-project/go-filecoin/issues/3025
func GetRecentAncestors(ctx context.Context, base block.TipSet, provider TipSetProvider, minHeight *types.BlockHeight) (ts []block.TipSet, err error) {
	ctx, span := trace.StartSpan(ctx, "Chain.GetRecentAncestors")
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	iterator := IterAncestors(ctx, provider, base)
	return CollectTipSetsPastHeight(iterator, minHeight)
}

// CollectTipSetsPastHeight collects all tipsets down to the first tipset with a height less than
// or equal to the earliest possible proving period start
func CollectTipSetsPastHeight(iterator *TipsetIterator, minHeight *types.BlockHeight) ([]block.TipSet, error) {
	var ret []block.TipSet
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
		ret = append(ret, iterator.Value())
		if types.NewBlockHeight(h).LessEqual(minHeight) {
			err = iterator.Next()
			if err != nil {
				return nil, err
			}
			break
		}
	}

	return ret, nil
}

// CollectAtMostNTipSets collect N tipsets from the input channel.  If there
// are fewer than n tipsets in the channel it returns all of them.
func CollectAtMostNTipSets(ctx context.Context, iterator *TipsetIterator, n uint) ([]block.TipSet, error) {
	var ret []block.TipSet
	var err error
	for i := uint(0); i < n && !iterator.Complete(); i++ {
		ret = append(ret, iterator.Value())
		if err = iterator.Next(); err != nil {
			return nil, err
		}
	}
	return ret, nil
}

// CollectTipSetsOfHeightAtLeast collects all tipsets with a height greater
// than or equal to minHeight from the input tipset.
func CollectTipSetsOfHeightAtLeast(ctx context.Context, iterator *TipsetIterator, minHeight *types.BlockHeight) ([]block.TipSet, error) {
	var ret []block.TipSet
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
			return ret, nil
		}
		ret = append(ret, iterator.Value())
	}
	return ret, nil
}

// FindCommonAncestor returns the common ancestor of the two tipsets pointed to
// by the input iterators.  If they share no common ancestor ErrNoCommonAncestor
// will be returned.
func FindCommonAncestor(leftIter, rightIter *TipsetIterator) (block.TipSet, error) {
	for !rightIter.Complete() && !leftIter.Complete() {
		left := leftIter.Value()
		right := rightIter.Value()

		leftHeight, err := left.Height()
		if err != nil {
			return block.UndefTipSet, err
		}
		rightHeight, err := right.Height()
		if err != nil {
			return block.UndefTipSet, err
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
				return block.UndefTipSet, err
			}
		}

		if leftHeight >= rightHeight {
			if err := leftIter.Next(); err != nil {
				return block.UndefTipSet, err
			}
		}
	}
	return block.UndefTipSet, ErrNoCommonAncestor
}
