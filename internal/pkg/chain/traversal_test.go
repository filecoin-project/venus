package chain_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

func TestIterAncestors(t *testing.T) {
	tf.UnitTest(t)
	miner, err := address.NewSecp256k1Address([]byte(fmt.Sprintf("address")))
	require.NoError(t, err)

	t.Run("iterates", func(t *testing.T) {
		ctx := context.Background()
		store := chain.NewBuilder(t, miner)

		root := store.AppendBlockOnBlocks()
		b11 := store.AppendBlockOnBlocks(root)
		b12 := store.AppendBlockOnBlocks(root)
		b21 := store.AppendBlockOnBlocks(b11, b12)

		t0 := th.RequireNewTipSet(t, root)
		t1 := th.RequireNewTipSet(t, b11, b12)
		t2 := th.RequireNewTipSet(t, b21)

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

		th.RequireNewTipSet(t, root)
		t1 := th.RequireNewTipSet(t, b11, b12)
		t2 := th.RequireNewTipSet(t, b21)

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
