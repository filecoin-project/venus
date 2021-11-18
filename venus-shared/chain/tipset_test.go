package chain

import (
	"testing"
	"time"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"

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
	assert.GreaterOrEqual(t, len(parents), minBlockHeaderNumForTest)
	assert.Less(t, len(parents), maxBlockHeaderNumForTest)

	var parentWeight big.Int
	testutil.Provide(t, &parentWeight, testutil.PositiveBigProvider())
	assert.True(t, parentWeight.GreaterThan(big.Zero()))

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

	assert.GreaterOrEqual(t, len(bhs), minBlockHeaderNumForTest)
	assert.Less(t, len(bhs), maxBlockHeaderNumForTest)

	for ai := 0; ai < len(appliers); ai++ {
		_, err := NewTipSet(bhs)
		assert.Errorf(t, err, "attempt to construct tipset before applier #%d", ai)
		assert.Containsf(t, err.Error(), appliers[ai].msg, "err msg content before applier #%d", ai)

		for bi := range bhs {
			appliers[ai].fn(bhs[bi])
		}
	}

	// duplicate bh
	_, err := NewTipSet(append(bhs, bhs[0]))
	assert.Error(t, err, "attempt to construct tipset with duplicated bh")
	assert.Containsf(t, err.Error(), "duplicate block ", "err msg content for duplicate block")

	// construct
	ts, err := NewTipSet(bhs)
	assert.NoError(t, err, "construct tipset")

	return ts
}

func TestTipSetConstruct(t *testing.T) {
	height, parents, parentWeight := constructTipSetKeyInfos(t)
	constructTipSet(t, height, parents, parentWeight)
}

func TestTipSetMethods(t *testing.T) {
	height, parents, parentWeight := constructTipSetKeyInfos(t)

	ts := constructTipSet(t, height, parents, parentWeight)
	assert.True(t, ts.Defined())

	tsk := ts.Key()
	assert.NotEqual(t, EmptyTSK, tsk, "tsk not empty")

	assert.Equal(t, ts.Height(), height)

	assert.True(t, ts.ParentWeight().Equals(parentWeight), "parent weight")

	child := constructTipSet(t, height+1, tsk.Cids(), BigMul(parentWeight, NewInt(2)))
	assert.True(t, child.IsChildOf(ts), "check if is child")
}
