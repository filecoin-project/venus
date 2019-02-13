package chn

import (
	"context"
	"testing"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
)

type FakeChainer struct {
	head    types.TipSet
	tipSets []types.TipSet
	blocks  map[cid.Cid]*types.Block
}

// BlockHistory returns the head of the chain tracked by the store.
func (mcr *FakeChainer) BlockHistory(ctx context.Context, start types.TipSet) <-chan interface{} {
	out := make(chan interface{}, len(mcr.tipSets))

	go func() {
		defer close(out)

		for _, tipSet := range mcr.tipSets {
			out <- tipSet
		}
	}()

	return out
}

// GetBlock returns a block by CID.
func (mcr FakeChainer) GetBlock(ctx context.Context, cid cid.Cid) (*types.Block, error) {
	blk, ok := mcr.blocks[cid]
	if !ok {
		return nil, errors.New("no such block")
	}
	return blk, nil
}

func (mcr *FakeChainer) Head() types.TipSet {
	return mcr.head
}

func TestChainLs(t *testing.T) {
	t.Parallel()
	t.Run("Head returns chain head", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		require := require.New(t)

		expected, err := types.NewTipSet(types.NewBlockForTest(nil, 2))
		require.NoError(err)

		chainAPI := New(&FakeChainer{
			head: expected,
		})

		head := chainAPI.Head(context.Background())
		assert.Equal(expected, head)
	})
	t.Run("Ls creates a channel of tipsets", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		require := require.New(t)

		expected1, err := types.NewTipSet(types.NewBlockForTest(nil, 2))
		require.NoError(err)

		expected2, err := types.NewTipSet(types.NewBlockForTest(nil, 3))
		require.NoError(err)

		chainAPI := New(&FakeChainer{
			tipSets: []types.TipSet{expected1, expected2},
		})

		ls := chainAPI.Ls(context.Background())

		actual1 := <-ls
		assert.Equal(expected1, actual1)

		actual2 := <-ls
		assert.Equal(expected2, actual2)
	})
	t.Run("BlockGet returns block", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		blk := types.NewBlockForTest(nil, 1)
		chainAPI := New(&FakeChainer{
			blocks: map[cid.Cid]*types.Block{blk.Cid(): blk},
		})

		_, err := chainAPI.BlockGet(context.Background(), types.SomeCid())
		assert.Error(err)

		found, err := chainAPI.BlockGet(context.Background(), blk.Cid())
		assert.NoError(err)
		assert.True(found.Equals(blk))
	})
}
