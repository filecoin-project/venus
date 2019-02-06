package chn

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockChainReader struct {
	tipSets []consensus.TipSet
}

// BlockHistory returns the head of the chain tracked by the store.
func (mcr *MockChainReader) BlockHistory(ctx context.Context) <-chan interface{} {
	out := make(chan interface{}, len(mcr.tipSets))

	go func() {
		defer close(out)

		for _, tipSet := range mcr.tipSets {
			out <- tipSet
		}
	}()

	return out
}

func TestChainLs(t *testing.T) {
	t.Parallel()
	t.Run("Ls creates a channel of tipsets", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		require := require.New(t)

		expected1, err := consensus.NewTipSet(types.NewBlockForTest(nil, 2))
		require.NoError(err)

		expected2, err := consensus.NewTipSet(types.NewBlockForTest(nil, 3))
		require.NoError(err)

		chainAPI := New(&MockChainReader{
			tipSets: []consensus.TipSet{expected1, expected2},
		})

		ls := chainAPI.Ls(context.Background())

		actual1 := <-ls
		assert.Equal(expected1, actual1)

		actual2 := <-ls
		assert.Equal(expected2, actual2)
	})
}
