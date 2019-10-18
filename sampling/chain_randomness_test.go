package sampling_test

import (
	"strconv"
	"testing"

	"github.com/filecoin-project/go-filecoin/block"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/sampling"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestSamplingChainRandomness(t *testing.T) {
	tf.UnitTest(t)

	t.Run("happy path", func(t *testing.T) {
		_, ch := makeChain(t, 21)
		r, err := sampling.SampleChainRandomness(types.NewBlockHeight(uint64(20)), ch)
		assert.NoError(t, err)
		assert.Equal(t, []byte(strconv.Itoa(20)), r)

		r, err = sampling.SampleChainRandomness(types.NewBlockHeight(uint64(3)), ch)
		assert.NoError(t, err)
		assert.Equal(t, []byte(strconv.Itoa(3)), r)

		r, err = sampling.SampleChainRandomness(types.NewBlockHeight(uint64(0)), ch)
		assert.NoError(t, err)
		assert.Equal(t, []byte(strconv.Itoa(0)), r)
	})

	t.Run("skips missing tipsets", func(t *testing.T) {
		builder, ch := makeChain(t, 21)

		// Sample height after the head falls back to the head
		r, err := sampling.SampleChainRandomness(types.NewBlockHeight(uint64(25)), ch)
		assert.NoError(t, err)
		assert.Equal(t, []byte(strconv.Itoa(20)), r)

		// Add new head so as to produce null blocks between 20 and 25
		// i.e.: 25 20 19 18 ... 0
		headAfterNulls := builder.BuildOneOn(ch[0], func(b *chain.BlockBuilder) {
			b.IncHeight(4)
			b.SetTicket(types.Signature(strconv.Itoa(25)))
		})
		ch = append([]block.TipSet{headAfterNulls}, ch...)

		// Sampling in the nulls falls back to the last non-null
		r, err = sampling.SampleChainRandomness(types.NewBlockHeight(uint64(24)), ch)
		assert.NoError(t, err)
		assert.Equal(t, []byte(strconv.Itoa(20)), r)

		// When sampling immediately after the nulls, the look-back skips the nulls (not counting them).
		r, err = sampling.SampleChainRandomness(types.NewBlockHeight(uint64(25)), ch)
		assert.NoError(t, err)
		assert.Equal(t, []byte(strconv.Itoa(25)), r)
	})

	t.Run("fails when chain insufficient", func(t *testing.T) {
		// Chain: 20, 19, 18, 17, 16
		// The final tipset is not of height zero (genesis)
		_, ch := makeChain(t, 21)
		ch = ch[:5]

		// Sample is out of range
		_, err := sampling.SampleChainRandomness(types.NewBlockHeight(uint64(15)), ch)
		assert.Error(t, err)

		// Ok when the chain is just sufficiently long.
		r, err := sampling.SampleChainRandomness(types.NewBlockHeight(uint64(16)), ch)
		assert.NoError(t, err)
		assert.Equal(t, []byte(strconv.Itoa(16)), r)
	})

	t.Run("falls back to genesis block", func(t *testing.T) {
		_, ch := makeChain(t, 6)

		// Sample height can be zero.
		r, err := sampling.SampleChainRandomness(types.NewBlockHeight(uint64(0)), ch)
		assert.NoError(t, err)
		assert.Equal(t, []byte(strconv.Itoa(0)), r)
	})
}

// Builds a chain of single-block tips, returned in descending height order.
// Each block's ticket is its stringified height (as bytes).
func makeChain(t *testing.T, length int) (*chain.Builder, []block.TipSet) {
	b := chain.NewBuilder(t, address.Undef)
	height := 0
	head := b.BuildManyOn(length, block.UndefTipSet, func(b *chain.BlockBuilder) {
		b.SetTicket(types.Signature(strconv.Itoa(height)))
		height++
	})
	return b, b.RequireTipSets(head.Key(), length)
}
