package chain

import (
	"testing"
	"time"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/venus-shared/testutil"
)

const (
	minBlockHeaderNumForTest = 3
	maxBlockHeaderNumForTest = 10
)

func constructTipSetKeyInfos(t *testing.T) (abi.ChainEpoch, []cid.Cid, big.Int) {
	now := time.Now().Unix()
	height := abi.ChainEpoch(now)

	var parentNum int
	testutil.Provide(t, &parentNum, testutil.IntRangedProvider(minBlockHeaderNumForTest, maxBlockHeaderNumForTest))

	var parents []cid.Cid
	testutil.Provide(t, &parents, testutil.WithSliceLen(parentNum))
	require.GreaterOrEqual(t, len(parents), minBlockHeaderNumForTest)
	require.Less(t, len(parents), maxBlockHeaderNumForTest)

	var parentWeight big.Int
	testutil.Provide(t, &parentWeight, testutil.PositiveBigProvider())
	require.True(t, parentWeight.GreaterThan(big.Zero()))

	return height, parents, parentWeight
}

func constructTipSet(t *testing.T, height abi.ChainEpoch, parents []cid.Cid, parentWeight big.Int) *TipSet {
	appliers := []struct {
		fn  func(*BlockHeader)
		msg string
	}{
		{
			fn: func(bh *BlockHeader) {
				bh.Height = height
			},
			msg: "inconsistent block heights ",
		},

		{
			fn: func(bh *BlockHeader) {
				bh.Parents = make([]cid.Cid, len(parents))
				copy(bh.Parents, parents)
			},
			msg: "inconsistent block parents ",
		},

		{
			fn: func(bh *BlockHeader) {
				bh.ParentWeight.Int.Set(parentWeight.Int)
			},
			msg: "inconsistent block parent weights ",
		},
	}

	var blkNum int
	testutil.Provide(t, &blkNum, testutil.IntRangedProvider(minBlockHeaderNumForTest, maxBlockHeaderNumForTest))

	var bhs []*BlockHeader
	testutil.Provide(t, &bhs, testutil.WithSliceLen(blkNum))

	require.GreaterOrEqual(t, len(bhs), minBlockHeaderNumForTest)
	require.Less(t, len(bhs), maxBlockHeaderNumForTest)

	for ai := 0; ai < len(appliers); ai++ {
		_, err := NewTipSet(bhs)
		require.Errorf(t, err, "attempt to construct tipset before applier #%d", ai)
		require.Containsf(t, err.Error(), appliers[ai].msg, "err msg content before applier #%d", ai)

		for bi := range bhs {
			appliers[ai].fn(bhs[bi])
		}
	}

	// duplicate bh
	_, err := NewTipSet(append(bhs, bhs[0]))
	require.Error(t, err, "attempt to construct tipset with duplicated bh")
	require.Containsf(t, err.Error(), "duplicate block ", "err msg content for duplicate block")

	// construct
	ts, err := NewTipSet(bhs)
	require.NoError(t, err, "construct tipset")

	return ts
}

func TestTipSetConstruct(t *testing.T) {
	height, parents, parentWeight := constructTipSetKeyInfos(t)
	constructTipSet(t, height, parents, parentWeight)
}

func TestTipSetMethods(t *testing.T) {
	height, parents, parentWeight := constructTipSetKeyInfos(t)

	ts := constructTipSet(t, height, parents, parentWeight)
	require.True(t, ts.Defined())

	tsk := ts.Key()
	require.NotEqual(t, EmptyTSK, tsk, "tsk not empty")

	require.Equal(t, ts.Height(), height)

	require.True(t, ts.ParentWeight().Equals(parentWeight), "parent weight")

	child := constructTipSet(t, height+1, tsk.Cids(), BigMul(parentWeight, NewInt(2)))
	require.True(t, child.IsChildOf(ts), "check if is child")
}
