package chain_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/chain"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
)

func TestSamplingChainRandomness(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	genesisTicket := block.Ticket{VRFProof: []byte{1, 2, 3, 4}}

	makeSample := func(sampleEpoch int) block.Ticket {
		vrfProof := genesisTicket.VRFProof
		if sampleEpoch >= 0 {
			vrfProof = []byte(strconv.Itoa(sampleEpoch))
		}
		return block.Ticket{
			VRFProof: vrfProof,
		}
	}

	t.Run("happy path", func(t *testing.T) {
		builder, ch := makeChain(t, 21)
		head := ch[0].Key()
		sampler := chain.NewSampler(builder, genesisTicket)

		r, err := sampler.SampleTicket(ctx, head, abi.ChainEpoch(20))
		assert.NoError(t, err)
		assert.Equal(t, makeSample(20), r)

		r, err = sampler.SampleTicket(ctx, head, abi.ChainEpoch(3))
		assert.NoError(t, err)
		assert.Equal(t, makeSample(3), r)

		r, err = sampler.SampleTicket(ctx, head, abi.ChainEpoch(0))
		assert.NoError(t, err)
		assert.Equal(t, makeSample(0), r)
	})

	t.Run("skips missing tipsets", func(t *testing.T) {
		builder, ch := makeChain(t, 21)
		sampler := chain.NewSampler(builder, genesisTicket)

		// Sample height after the head falls back to the head.
		headParent := ch[1].Key()
		r, err := sampler.SampleTicket(ctx, headParent, abi.ChainEpoch(20))
		assert.EqualError(t, err, "cannot draw randomness from the future")

		// Another way of the same thing, sample > head.
		head := ch[0].Key()
		r, err = sampler.SampleTicket(ctx, head, abi.ChainEpoch(21))
		assert.EqualError(t, err, "cannot draw randomness from the future")

		// Add new head so as to produce null blocks between 20 and 25
		// i.e.: 25 20 19 18 ... 0
		headAfterNulls := builder.BuildOneOn(ch[0], func(b *chain.BlockBuilder) {
			b.IncHeight(4)
			b.SetTicket([]byte(strconv.Itoa(25)))
		})

		// Sampling in the nulls falls back to the last non-null
		r, err = sampler.SampleTicket(ctx, headAfterNulls.Key(), abi.ChainEpoch(24))
		assert.NoError(t, err)
		assert.Equal(t, makeSample(20), r)
	})

	t.Run("genesis", func(t *testing.T) {
		builder, ch := makeChain(t, 6)
		head := ch[0].Key()
		gen := (ch[len(ch)-1]).Key()
		sampler := chain.NewSampler(builder, genesisTicket)

		// Sample genesis from longer chain.
		r, err := sampler.SampleTicket(ctx, head, abi.ChainEpoch(0))
		assert.NoError(t, err)
		assert.Equal(t, makeSample(0), r)

		// Sample before genesis from longer chain.
		r, err = sampler.SampleTicket(ctx, head, abi.ChainEpoch(-1))
		assert.NoError(t, err)
		assert.Equal(t, makeSample(0), r)

		// Sample genesis from genesis-only chain.
		r, err = sampler.SampleTicket(ctx, gen, abi.ChainEpoch(0))
		assert.NoError(t, err)
		assert.Equal(t, makeSample(0), r)

		// Sample before genesis from genesis-only chain.
		r, err = sampler.SampleTicket(ctx, gen, abi.ChainEpoch(-1))
		assert.NoError(t, err)
		assert.Equal(t, makeSample(0), r)

		// Sample empty chain.
		r, err = sampler.SampleTicket(ctx, block.NewTipSetKey(), abi.ChainEpoch(0))
		assert.NoError(t, err)
		assert.Equal(t, makeSample(-1), r)
		r, err = sampler.SampleTicket(ctx, block.NewTipSetKey(), abi.ChainEpoch(-1))
		assert.NoError(t, err)
		assert.Equal(t, makeSample(-1), r)
	})
}

// Builds a chain of single-block tips, returned in descending height order.
// Each block's ticket is its stringified height (as bytes).
func makeChain(t *testing.T, length int) (*chain.Builder, []*block.TipSet) {
	b := chain.NewBuilder(t, address.Undef)
	height := 0
	head := b.BuildManyOn(length, block.UndefTipSet, func(b *chain.BlockBuilder) {
		b.SetTicket([]byte(strconv.Itoa(height)))
		height++
	})
	return b, b.RequireTipSets(head.Key(), length)
}
