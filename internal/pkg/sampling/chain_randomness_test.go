package sampling_test

import (
	"strconv"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/sampling"
)

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
		b.SetTicket([]byte(strconv.Itoa(height)))
		height++
	})
	return b, b.RequireTipSets(head.Key(), length)
}
