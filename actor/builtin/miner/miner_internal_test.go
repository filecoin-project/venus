package miner

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestLatePoStFee(t *testing.T) {
	tf.UnitTest(t)
	pledgeCollateral := af(1000)

	t.Run("on time charges no fee", func(t *testing.T) {
		assert.True(t, types.ZeroAttoFIL.Equal(latePoStFee(pledgeCollateral, bh(1000), bh(999), bh(100))))
		assert.True(t, types.ZeroAttoFIL.Equal(latePoStFee(pledgeCollateral, bh(1000), bh(1000), bh(100))))
	})

	t.Run("fee proportional to lateness", func(t *testing.T) {
		// 1 block late is 1% of 100 allowable
		assert.True(t, af(10).Equal(latePoStFee(pledgeCollateral, bh(1000), bh(1001), bh(100))))
		// 5 blocks late of 100 allowable
		assert.True(t, af(50).Equal(latePoStFee(pledgeCollateral, bh(1000), bh(1005), bh(100))))

		// 2 blocks late of 10000 allowable, fee rounds down to zero
		assert.True(t, af(0).Equal(latePoStFee(pledgeCollateral, bh(1000), bh(1002), bh(10000))))
		// 9 blocks late of 10000 allowable, fee rounds down to zero
		assert.True(t, af(0).Equal(latePoStFee(pledgeCollateral, bh(1000), bh(1009), bh(10000))))
		// 10 blocks late of 10000 allowable is 1/1000
		assert.True(t, af(1).Equal(latePoStFee(pledgeCollateral, bh(1000), bh(1010), bh(10000))))
	})

	t.Run("fee capped at total pledge", func(t *testing.T) {
		assert.True(t, pledgeCollateral.Equal(latePoStFee(pledgeCollateral, bh(1000), bh(1100), bh(100))))
		assert.True(t, pledgeCollateral.Equal(latePoStFee(pledgeCollateral, bh(1000), bh(2000), bh(100))))
	})
}

func af(h int64) types.AttoFIL {
	return types.NewAttoFIL(big.NewInt(h))
}

func bh(h uint64) *types.BlockHeight {
	return types.NewBlockHeight(uint64(h))
}
