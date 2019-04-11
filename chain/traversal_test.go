package chain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/chain"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestGetParentTipSet(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)
	ctx := context.Background()
	store := th.NewFakeBlockProvider()

	root := store.NewBlock(0)
	b11 := store.NewBlock(1, root)
	b12 := store.NewBlock(2, root)
	b21 := store.NewBlock(3, b11, b12)

	t.Run("root has empty parent", func(t *testing.T) {
		ts := requireTipset(t, root)
		parent, e := chain.GetParentTipSet(ctx, store, ts)
		require.NoError(e)
		assert.Empty(parent)
	})
	t.Run("plural tipset", func(t *testing.T) {
		ts := requireTipset(t, b11, b12)
		parent, e := chain.GetParentTipSet(ctx, store, ts)
		require.NoError(e)
		assert.True(requireTipset(t, root).Equals(parent))
	})
	t.Run("plural parent", func(t *testing.T) {
		ts := requireTipset(t, b21)
		parent, e := chain.GetParentTipSet(ctx, store, ts)
		require.NoError(e)
		assert.True(requireTipset(t, b11, b12).Equals(parent))
	})
}

func TestIterAncestors(t *testing.T) {
	t.Parallel()

	t.Run("iterates", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		ctx := context.Background()
		store := th.NewFakeBlockProvider()

		root := store.NewBlock(0)
		b11 := store.NewBlock(1, root)
		b12 := store.NewBlock(2, root)
		b21 := store.NewBlock(3, b11, b12)

		t0 := requireTipset(t, root)
		t1 := requireTipset(t, b11, b12)
		t2 := requireTipset(t, b21)

		it := chain.IterAncestors(ctx, store, t2)
		assert.False(it.Complete())
		assert.True(t2.Equals(it.Value()))

		assert.NoError(it.Next())
		assert.False(it.Complete())
		assert.True(t1.Equals(it.Value()))

		assert.NoError(it.Next())
		assert.False(it.Complete())
		assert.True(t0.Equals(it.Value()))

		assert.NoError(it.Next())
		assert.True(it.Complete())
	})

	t.Run("respects context", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		ctx, cancel := context.WithCancel(context.Background())
		store := th.NewFakeBlockProvider()

		root := store.NewBlock(0)
		b11 := store.NewBlock(1, root)
		b12 := store.NewBlock(2, root)
		b21 := store.NewBlock(3, b11, b12)

		requireTipset(t, root)
		t1 := requireTipset(t, b11, b12)
		t2 := requireTipset(t, b21)

		it := chain.IterAncestors(ctx, store, t2)
		assert.False(it.Complete())
		assert.True(t2.Equals(it.Value()))

		assert.NoError(it.Next())
		assert.False(it.Complete())
		assert.True(t1.Equals(it.Value()))

		cancel()

		assert.Error(it.Next())
	})
}

func requireTipset(t *testing.T, blocks ...*types.Block) types.TipSet {
	set, err := types.NewTipSet(blocks...)
	require.NoError(t, err)
	return set
}
