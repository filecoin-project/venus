package sampling_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/sampling"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestSamplingChainRandomness(t *testing.T) {
	tf.UnitTest(t)

	// set a tripwire
	require.Equal(t, sampling.LookbackParameter, 3, "these tests assume LookbackParameter=3")

	t.Run("happy path", func(t *testing.T) {
		// The tipsets are in descending height order. Each block's ticket is its stringified height (as bytes).
		chain := testhelpers.RequireTipSetChain(t, 20)

		r, err := sampling.SampleChainRandomness(types.NewBlockHeight(uint64(20)), chain)
		assert.NoError(t, err)
		assert.Equal(t, []byte(strconv.Itoa(17)), r)

		r, err = sampling.SampleChainRandomness(types.NewBlockHeight(uint64(3)), chain)
		assert.NoError(t, err)
		assert.Equal(t, []byte(strconv.Itoa(0)), r)

		r, err = sampling.SampleChainRandomness(types.NewBlockHeight(uint64(10)), chain)
		assert.NoError(t, err)
		assert.Equal(t, []byte(strconv.Itoa(7)), r)
	})

	t.Run("skips missing tipsets", func(t *testing.T) {
		chain := testhelpers.RequireTipSetChain(t, 20)

		// Sample height after the head falls back to the head, and then looks back from there
		r, err := sampling.SampleChainRandomness(types.NewBlockHeight(uint64(25)), chain)
		assert.NoError(t, err)
		assert.Equal(t, []byte(strconv.Itoa(17)), r)

		// Add new head so as to produce null blocks between 20 and 25
		// i.e.: 25 20 19 18 ... 0
		headAfterNulls := types.NewBlockForTest(chain[0].ToSlice()[0], uint64(0))
		headAfterNulls.Height = types.Uint64(uint64(25))
		headAfterNulls.Ticket = []byte(strconv.Itoa(int(headAfterNulls.Height)))
		chain = append([]types.TipSet{types.RequireNewTipSet(t, headAfterNulls)}, chain...)

		// Sampling in the nulls falls back to the last non-null
		r, err = sampling.SampleChainRandomness(types.NewBlockHeight(uint64(24)), chain)
		assert.NoError(t, err)
		assert.Equal(t, []byte(strconv.Itoa(17)), r)

		// When sampling immediately after the nulls, the look-back skips the nulls (not counting them).
		r, err = sampling.SampleChainRandomness(types.NewBlockHeight(uint64(25)), chain)
		assert.NoError(t, err)
		assert.Equal(t, []byte(strconv.Itoa(18)), r)
	})

	t.Run("fails when chain insufficient", func(t *testing.T) {
		// Chain: 20, 19, 18, 17, 16
		// The final tipset is not of height zero (genesis)
		chain := testhelpers.RequireTipSetChain(t, 20)[:5]

		// Sample is out of range
		_, err := sampling.SampleChainRandomness(types.NewBlockHeight(uint64(15)), chain)
		assert.Error(t, err)

		// Sample minus lookback is out of range
		_, err = sampling.SampleChainRandomness(types.NewBlockHeight(uint64(16)), chain)
		assert.Error(t, err)
		_, err = sampling.SampleChainRandomness(types.NewBlockHeight(uint64(18)), chain)
		assert.Error(t, err)

		// Ok when the chain is just sufficiently long.
		r, err := sampling.SampleChainRandomness(types.NewBlockHeight(uint64(19)), chain)
		assert.NoError(t, err)
		assert.Equal(t, []byte(strconv.Itoa(16)), r)
	})

	t.Run("falls back to genesis block", func(t *testing.T) {
		chain := testhelpers.RequireTipSetChain(t, 5)

		// Three blocks back from "1"
		r, err := sampling.SampleChainRandomness(types.NewBlockHeight(uint64(1)), chain)
		assert.NoError(t, err)
		assert.Equal(t, []byte(strconv.Itoa(0)), r)

		// Sample height can be zero.
		r, err = sampling.SampleChainRandomness(types.NewBlockHeight(uint64(0)), chain)
		assert.NoError(t, err)
		assert.Equal(t, []byte(strconv.Itoa(0)), r)
	})
}
