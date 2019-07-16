package chain

import (
	"context"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/go-filecoin/metrics/tracing"
	"github.com/filecoin-project/go-filecoin/types"
)

// ErrNoCommonAncestor is returned when two chains assumed to have a common ancestor do not.
var ErrNoCommonAncestor = errors.New("no common ancestor")

// GetRecentAncestors returns the ancestors of base as a slice of TipSets.
//
// In order to validate post messages, randomness from the chain is required.
// This function collects that randomess: all tipsets with height greater than
// childBH - ancestorRounds, and the lookback tipsets that precede them.
//
// The return slice is a concatenation of two slices: append(provingPeriodAncestors, extraRandomnessAncestors...)
//   provingPeriodAncestors: all ancestor tipsets with height greater than childBH - ancestorRoundsNeeded
//   extraRandomnessAncestors: the lookback number of tipsets directly preceding tipsets in provingPeriodAncestors
//
// The last tipset of provingPeriodAncestors is the earliest possible tipset to
// begin a proving period that is still "live", i.e it is valid to accept PoSts
// over this proving period when processing a tipset at childBH.  The last
// tipset of extraRandomnessAncestors is the tipset used to sample randomness
// for any PoSts with a proving period beginning at the last tipset of
// provingPeriodAncestors.  By including ancestors as far back as the last tipset
// of extraRandomnessAncestors, the consensus state transition function can sample
// the randomness used by all live PoSts to correctly process all valid
// 'submitPoSt' messages.
//
// Because null blocks increase chain height but do not have associated tipsets
// the length of provingPeriodAncestors may vary (more null blocks -> shorter length).  The
// length of slice extraRandomnessAncestors is a constant (at least once the
// chain is longer than lookback tipsets).
// This is all more complex than necessary, we should just index tipsets by height:
// https://github.com/filecoin-project/go-filecoin/issues/3025
func GetRecentAncestors(ctx context.Context, base types.TipSet, provider TipSetProvider, childBH, ancestorRoundsNeeded *types.BlockHeight, lookback uint) (ts []types.TipSet, err error) {
	ctx, span := trace.StartSpan(ctx, "Chain.GetRecentAncestors")
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	if lookback == 0 {
		return nil, errors.New("lookback must be greater than 0")
	}
	earliestAncestorHeight := childBH.Sub(ancestorRoundsNeeded)
	if earliestAncestorHeight.LessThan(types.NewBlockHeight(0)) {
		earliestAncestorHeight = types.NewBlockHeight(uint64(0))
	}

	// Step 1 -- gather all tipsets with a height greater than the earliest
	// possible proving period start still in scope for the given head.
	iterator := IterAncestors(ctx, provider, base)
	provingPeriodAncestors, err := CollectTipSetsOfHeightAtLeast(ctx, iterator, earliestAncestorHeight)
	if err != nil {
		return nil, err
	}
	firstExtraRandomnessAncestorsCids, err := provingPeriodAncestors[len(provingPeriodAncestors)-1].Parents()
	if err != nil {
		return nil, err
	}
	// no parents means hit genesis so return the whole chain
	if firstExtraRandomnessAncestorsCids.Len() == 0 {
		return provingPeriodAncestors, nil
	}

	// Step 2 -- gather the lookback tipsets directly preceding provingPeriodAncestors.
	lookBackTS, err := provider.GetTipSet(firstExtraRandomnessAncestorsCids)
	if err != nil {
		return nil, err
	}
	iterator = IterAncestors(ctx, provider, lookBackTS)
	extraRandomnessAncestors, err := CollectAtMostNTipSets(ctx, iterator, lookback)
	if err != nil {
		return nil, err
	}
	return append(provingPeriodAncestors, extraRandomnessAncestors...), nil
}

// CollectTipSetsOfHeightAtLeast collects all tipsets with a height greater
// than or equal to minHeight from the input tipset.
func CollectTipSetsOfHeightAtLeast(ctx context.Context, iterator *TipsetIterator, minHeight *types.BlockHeight) ([]types.TipSet, error) {
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
			return ret, nil
		}
		ret = append(ret, iterator.Value())
	}
	return ret, nil
}

// CollectAtMostNTipSets collect N tipsets from the input channel.  If there
// are fewer than n tipsets in the channel it returns all of them.
func CollectAtMostNTipSets(ctx context.Context, iterator *TipsetIterator, n uint) ([]types.TipSet, error) {
	var ret []types.TipSet
	var err error
	for i := uint(0); i < n && !iterator.Complete(); i++ {
		ret = append(ret, iterator.Value())
		if err = iterator.Next(); err != nil {
			return nil, err
		}
	}
	return ret, nil
}

// FindCommonAncestor returns the common ancestor of the two tipsets pointed to
// by the input iterators.  If they share no common ancestor ErrNoCommonAncestor
// will be returned.
func FindCommonAncestor(leftIter, rightIter *TipsetIterator) (types.TipSet, error) {
	for !rightIter.Complete() && !leftIter.Complete() {
		left := leftIter.Value()
		right := rightIter.Value()

		leftHeight, err := left.Height()
		if err != nil {
			return types.UndefTipSet, err
		}
		rightHeight, err := right.Height()
		if err != nil {
			return types.UndefTipSet, err
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
				return types.UndefTipSet, err
			}
		}

		if leftHeight >= rightHeight {
			if err := leftIter.Next(); err != nil {
				return types.UndefTipSet, err
			}
		}
	}
	return types.UndefTipSet, ErrNoCommonAncestor
}
