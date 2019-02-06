package impl

import (
	"context"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"testing"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChainHead(t *testing.T) {
	t.Parallel()
	t.Run("returns an error if no best block", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)

		n := node.MakeOfflineNode(t)
		api := New(n)

		_, err := api.Chain().Head()

		require.Error(err)
		require.EqualError(err, ErrHeaviestTipSetNotFound.Error())
	})

	t.Run("emits the blockchain head", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		require := require.New(t)
		assert := assert.New(t)

		blk := types.NewBlockForTest(nil, 1)
		n := node.MakeOfflineNode(t)
		chainStore, ok := n.ChainReader.(chain.Store)
		require.True(ok)

		chainStore.SetHead(ctx, testhelpers.RequireNewTipSet(require, blk))

		api := New(n)
		out, err := api.Chain().Head()

		require.NoError(err)
		assert.Len(out, 1)
		types.AssertCidsEqual(assert, out[0], blk.Cid())
	})

	t.Run("the blockchain head is sorted", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		require := require.New(t)
		assert := assert.New(t)

		blk := types.NewBlockForTest(nil, 0)
		blk2 := types.NewBlockForTest(nil, 1)
		blk3 := types.NewBlockForTest(nil, 2)

		n := node.MakeOfflineNode(t)
		chainStore, ok := n.ChainReader.(chain.Store)
		require.True(ok)

		newTipSet := testhelpers.RequireNewTipSet(require, blk)
		testhelpers.RequireTipSetAdd(require, blk2, newTipSet)
		testhelpers.RequireTipSetAdd(require, blk3, newTipSet)

		someErr := chainStore.SetHead(ctx, newTipSet)
		require.NoError(someErr)

		api := New(n)
		out, err := api.Chain().Head()
		require.NoError(err)

		sortedCidSet := types.NewSortedCidSet(blk.Cid(), blk2.Cid(), blk3.Cid())

		assert.Len(out, 3)
		assert.Equal(sortedCidSet.ToSlice(), out)
	})
}
