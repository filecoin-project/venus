package chain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/chain"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestIterAncestors(t *testing.T) {
	tf.UnitTest(t)

	t.Run("iterates", func(t *testing.T) {
		ctx := context.Background()
		store := th.NewFakeChainProvider()

		root := store.NewBlock(0)
		b11 := store.NewBlock(1, root)
		b12 := store.NewBlock(2, root)
		b21 := store.NewBlock(3, b11, b12)

		t0 := requireTipset(t, root)
		t1 := requireTipset(t, b11, b12)
		t2 := requireTipset(t, b21)

		it := chain.IterAncestors(ctx, store, t2)
		assert.False(t, it.Complete())
		assert.True(t, t2.Equals(it.Value()))

		assert.NoError(t, it.Next())
		assert.False(t, it.Complete())
		assert.True(t, t1.Equals(it.Value()))

		assert.NoError(t, it.Next())
		assert.False(t, it.Complete())
		assert.True(t, t0.Equals(it.Value()))

		assert.NoError(t, it.Next())
		assert.True(t, it.Complete())
	})

	t.Run("respects context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		store := th.NewFakeChainProvider()

		root := store.NewBlock(0)
		b11 := store.NewBlock(1, root)
		b12 := store.NewBlock(2, root)
		b21 := store.NewBlock(3, b11, b12)

		requireTipset(t, root)
		t1 := requireTipset(t, b11, b12)
		t2 := requireTipset(t, b21)

		it := chain.IterAncestors(ctx, store, t2)
		assert.False(t, it.Complete())
		assert.True(t, t2.Equals(it.Value()))

		assert.NoError(t, it.Next())
		assert.False(t, it.Complete())
		assert.True(t, t1.Equals(it.Value()))

		cancel()

		assert.Error(t, it.Next())
	})
}

func requireTipset(t *testing.T, blocks ...*types.Block) types.TipSet {
	set, err := types.NewTipSet(blocks...)
	require.NoError(t, err)
	return set
}
