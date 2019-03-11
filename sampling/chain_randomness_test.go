package sampling_test

import (
	"strconv"
	"testing"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"

	"github.com/filecoin-project/go-filecoin/sampling"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestSamplingChainRandomness(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	// set a tripwire
	require.Equal(sampling.LookbackParameter, 3, "these tests assume LookbackParameter=3")

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()
		chain := testhelpers.RequireTipSetChain(t, 20)

		r, err := sampling.SampleChainRandomness(types.NewBlockHeight(uint64(20)), chain)
		assert.NoError(err)
		assert.Equal([]byte(strconv.Itoa(17)), r)

		r, err = sampling.SampleChainRandomness(types.NewBlockHeight(uint64(3)), chain)
		assert.NoError(err)
		assert.Equal([]byte(strconv.Itoa(0)), r)

		r, err = sampling.SampleChainRandomness(types.NewBlockHeight(uint64(10)), chain)
		assert.NoError(err)
		assert.Equal([]byte(strconv.Itoa(7)), r)
	})

	t.Run("faults with height out of range", func(t *testing.T) {
		t.Parallel()
		chain := testhelpers.RequireTipSetChain(t, 20)

		// edit chain to include null blocks at heights 21 through 24
		baseBlock := chain[1].ToSlice()[0]
		afterNull := types.NewBlockForTest(baseBlock, uint64(0))
		afterNull.Height += types.Uint64(uint64(5))
		afterNull.Ticket = []byte(strconv.Itoa(int(afterNull.Height)))
		chain = append([]types.TipSet{types.RequireNewTipSet(require, afterNull)}, chain...)

		// ancestor block heights:
		//
		// 25 20 19 18 17 16 15 14 13 12 11 10 9 8 7 6 5 4 3 2 1 0
		//
		// no tip set with height 30 exists in ancestors
		_, err := sampling.SampleChainRandomness(types.NewBlockHeight(uint64(30)), chain)
		assert.Error(err)
	})

	t.Run("faults with lookback out of range", func(t *testing.T) {
		t.Parallel()
		chain := testhelpers.RequireTipSetChain(t, 20)[:5]

		// ancestor block heights:
		//
		// 20, 19, 18, 17, 16
		//
		// going back in time by `LookbackParameter`-number of tip sets from
		// block height 17 does not find us the genesis block
		_, err := sampling.SampleChainRandomness(types.NewBlockHeight(uint64(17)), chain)
		assert.Error(err)
	})

	t.Run("falls back to genesis block", func(t *testing.T) {
		t.Parallel()
		chain := testhelpers.RequireTipSetChain(t, 5)

		// ancestor block heights:
		//
		// 5, 3, 2, 1, 0
		//
		// going back in time by `LookbackParameter`-number of tip sets from 1
		// would put us into the negative - so fall back to genesis block
		r, err := sampling.SampleChainRandomness(types.NewBlockHeight(uint64(1)), chain)
		assert.NoError(err)
		assert.Equal([]byte(strconv.Itoa(0)), r)
	})
}
