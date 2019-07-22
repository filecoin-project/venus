package chain_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestIterAncestors(t *testing.T) {
	tf.UnitTest(t)
	miner, err := address.NewActorAddress([]byte(fmt.Sprintf("address")))
	require.NoError(t, err)

	t.Run("iterates", func(t *testing.T) {
		ctx := context.Background()
		store := chain.NewBuilder(t, miner)

		root := store.AppendBlockOnBlocks()
		b11 := store.AppendBlockOnBlocks(root)
		b12 := store.AppendBlockOnBlocks(root)
		b21 := store.AppendBlockOnBlocks(b11, b12)

		t0 := types.RequireNewTipSet(t, root)
		t1 := types.RequireNewTipSet(t, b11, b12)
		t2 := types.RequireNewTipSet(t, b21)

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
		store := chain.NewBuilder(t, miner)

		root := store.AppendBlockOnBlocks()
		b11 := store.AppendBlockOnBlocks(root)
		b12 := store.AppendBlockOnBlocks(root)
		b21 := store.AppendBlockOnBlocks(b11, b12)

		types.RequireNewTipSet(t, root)
		t1 := types.RequireNewTipSet(t, b11, b12)
		t2 := types.RequireNewTipSet(t, b21)

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
