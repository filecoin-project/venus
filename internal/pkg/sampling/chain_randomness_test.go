package sampling_test

import (
	"strconv"
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/sampling"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
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

func TestSampleNthTicket(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		_, ch := makeChain(t, 10)
		r, err := sampling.SampleNthTicket(3, ch)
		assert.NoError(t, err)
		// 9 - 3 = 6
		assert.Equal(t, block.VRFPi(strconv.Itoa(6)), r.VRFProof)
	})

	t.Run("falls back to genesis", func(t *testing.T) {
		_, ch := makeChain(t, 4)
		r, err := sampling.SampleNthTicket(5, ch)
		assert.NoError(t, err)
		assert.Equal(t, block.VRFPi(strconv.Itoa(0)), r.VRFProof)
	})

	t.Run("not enough tickets", func(t *testing.T) {
		_, ch := makeChain(t, 10)
		ch = ch[:5]
		_, err := sampling.SampleNthTicket(5, ch)
		assert.Error(t, err)
	})

	t.Run("empty ancestors array", func(t *testing.T) {
		_, err := sampling.SampleNthTicket(5, nil)
		assert.Error(t, err)
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
