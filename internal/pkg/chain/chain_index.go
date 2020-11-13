package chain

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/internal/pkg/block"
	lru "github.com/hashicorp/golang-lru"
	xerrors "github.com/pkg/errors"
)

var DefaultChainIndexCacheSize = 32 << 10

type ChainIndex struct { //nolint
	skipCache *lru.ARCCache

	loadTipSet loadTipSetFunc

	skipLength abi.ChainEpoch
}
type loadTipSetFunc func(block.TipSetKey) (*block.TipSet, error)

func NewChainIndex(lts loadTipSetFunc) *ChainIndex {
	sc, _ := lru.NewARC(DefaultChainIndexCacheSize)
	return &ChainIndex{
		skipCache:  sc,
		loadTipSet: lts,
		skipLength: 20,
	}
}

type lbEntry struct {
	ts           *block.TipSet
	parentHeight abi.ChainEpoch
	targetHeight abi.ChainEpoch
	target       block.TipSetKey
}

func (ci *ChainIndex) GetTipSetByHeight(_ context.Context, from *block.TipSet, to abi.ChainEpoch) (*block.TipSet, error) {
	if from.EnsureHeight()-to <= ci.skipLength {
		return ci.walkBack(from, to)
	}

	rounded, err := ci.roundDown(from)
	if err != nil {
		return nil, err
	}

	cur := rounded.Key()
	// cur := from.Key()
	for {
		cval, ok := ci.skipCache.Get(cur.String())
		if !ok {
			fc, err := ci.fillCache(cur)
			if err != nil {
				return nil, err
			}
			cval = fc
		}

		lbe := cval.(*lbEntry)
		if lbe.ts.EnsureHeight() == to || lbe.parentHeight < to {
			return lbe.ts, nil
		} else if to > lbe.targetHeight {
			return ci.walkBack(lbe.ts, to)
		}

		cur = lbe.target
	}
}

func (ci *ChainIndex) GetTipsetByHeightWithoutCache(from *block.TipSet, to abi.ChainEpoch) (*block.TipSet, error) {
	return ci.walkBack(from, to)
}

func (ci *ChainIndex) fillCache(tsk block.TipSetKey) (*lbEntry, error) {
	ts, err := ci.loadTipSet(tsk)
	if err != nil {
		return nil, err
	}

	if ts.EnsureHeight() == 0 {
		return &lbEntry{
			ts:           ts,
			parentHeight: 0,
		}, nil
	}

	// will either be equal to ts.Height, or at least > ts.Parent.Height()
	rheight := ci.roundHeight(ts.EnsureHeight())

	parent, err := ci.loadTipSet(ts.EnsureParents())
	if err != nil {
		return nil, err
	}

	rheight -= ci.skipLength

	var skipTarget *block.TipSet
	if parent.EnsureHeight() < rheight {
		skipTarget = parent
	} else {
		skipTarget, err = ci.walkBack(parent, rheight)
		if err != nil {
			return nil, xerrors.Errorf("fillCache walkback: %s", err)
		}
	}

	lbe := &lbEntry{
		ts:           ts,
		parentHeight: parent.EnsureHeight(),
		targetHeight: skipTarget.EnsureHeight(),
		target:       skipTarget.Key(),
	}
	ci.skipCache.Add(tsk.String(), lbe)

	return lbe, nil
}

// floors to nearest skipLength multiple
func (ci *ChainIndex) roundHeight(h abi.ChainEpoch) abi.ChainEpoch {
	return (h / ci.skipLength) * ci.skipLength
}

func (ci *ChainIndex) roundDown(ts *block.TipSet) (*block.TipSet, error) {
	target := ci.roundHeight(ts.EnsureHeight())

	rounded, err := ci.walkBack(ts, target)
	if err != nil {
		return nil, err
	}

	return rounded, nil
}

func (ci *ChainIndex) walkBack(from *block.TipSet, to abi.ChainEpoch) (*block.TipSet, error) {
	if to > from.EnsureHeight() {
		return nil, xerrors.Errorf("looking for tipset with height greater than start point")
	}

	if to == from.EnsureHeight() {
		return from, nil
	}

	ts := from

	for {
		pts, err := ci.loadTipSet(ts.EnsureParents())
		if err != nil {
			return nil, err
		}

		if to > pts.EnsureHeight() {
			// in case pts is lower than the epoch we're looking for (null blocks)
			// return a tipset above that height
			return ts, nil
		}
		if to == pts.EnsureHeight() {
			return pts, nil
		}

		ts = pts
	}
}
