package types

import (
	"math"
	"testing"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	"github.com/filecoin-project/venus/venus-shared/types/params"
	"github.com/ipfs/go-cid"
)

func init() {
	testutil.MustRegisterDefaultValueProvier(TipsetProvider())
	testutil.MustRegisterDefaultValueProvier(MessageProvider())
}

func TipsetProvider() func(*testing.T) *TipSet {
	const (
		minBlkNumInTipset = 1
		maxBlkNumInTipset = 5
	)

	return func(t *testing.T) *TipSet {
		var (
			blkNum, parentNum int
			parentWeight      big.Int
			epoch             abi.ChainEpoch
			blocks            []*BlockHeader
			parents           []cid.Cid
		)

		testutil.Provide(t, &parentNum, testutil.IntRangedProvider(minBlkNumInTipset, maxBlkNumInTipset))
		testutil.Provide(t, &parents, testutil.WithSliceLen(parentNum))

		testutil.Provide(t, &blkNum, testutil.IntRangedProvider(minBlkNumInTipset+1, maxBlkNumInTipset))
		testutil.Provide(t, &blocks, testutil.WithSliceLen(blkNum),
			// blocks in one tipset must be with the same parents.
			func(t *testing.T) []cid.Cid {
				return parents
			})

		testutil.Provide(t, &epoch, testutil.IntRangedProvider(0, math.MaxUint32))
		testutil.Provide(t, &parentWeight, testutil.PositiveBigProvider())

		// ensure that random assignments won't break the validation
		for _, blk := range blocks {
			blk.Height = epoch
			blk.ParentWeight.Set(parentWeight.Int)
		}

		tipset, err := NewTipSet(blocks)

		if err != nil {
			t.Fatalf("create new tipset failed: %s", err.Error())
		}

		return tipset
	}
}

func MessageProvider() func(t *testing.T) *Message {
	return func(t *testing.T) *Message {
		var msg Message
		testutil.Provide(t, &msg,
			testutil.IntRangedProvider(0, params.BlockGasLimit),
			func(t *testing.T) big.Int {
				ip := testutil.IntRangedProvider(0, int(params.FilBase))
				return FromFil(uint64(ip(t)))
			},
		)
		// ensure that random assignments won't break the validation
		msg.Version = 0
		msg.GasPremium = msg.GasFeeCap
		return &msg
	}
}
