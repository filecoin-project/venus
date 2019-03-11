package actr

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"

	"gx/ipfs/QmNf3wujpV2Y7Lnj2hy2UrmuX8bhMDStRHbnSLh7Ypf36h/go-hamt-ipld"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
	"gx/ipfs/QmRu7tiRnFk9mMPpVECQTBQJqXtmG132jJxA1w9A7TtpBz/go-ipfs-blockstore"
)

func TestActorLs(t *testing.T) {
	t.Parallel()

	t.Run("returns each actor and address", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		require := require.New(t)
		ctx := context.Background()

		cst := hamt.NewCborStore()
		repo := repo.NewInMemoryRepo()
		bs := blockstore.NewBlockstore(repo.Datastore())

		genBlock, err := consensus.DefaultGenesis(cst, bs)
		require.NoError(err)
		b1 := types.NewBlockForTest(genBlock, 1)
		ts := testhelpers.RequireNewTipSet(require, b1)

		chainStore := chain.NewDefaultStore(repo.ChainDatastore(), cst, genBlock.Cid())

		require.NoError(chainStore.PutTipSetAndState(ctx, &chain.TipSetAndState{
			TipSet:          ts,
			TipSetStateRoot: genBlock.StateRoot,
		}))

		require.NoError(chainStore.SetHead(ctx, testhelpers.RequireNewTipSet(require, b1)))

		actr := NewActor(chainStore)
		addrs, actors, err := actr.Ls(ctx)
		require.NoError(err)

		latestState, err := chainStore.LatestState(ctx)
		require.NoError(err)
		expectedAddrs, expectedActors := state.GetAllActors(latestState)

		assert.Equal(5, len(addrs))
		assert.Equal(5, len(actors))
		for i := 0; i < 5; i++ {
			assert.Equal(expectedAddrs[i], addrs[i])
			assert.Equal(expectedActors[i], actors[i])
		}
	})

	t.Run("returns an error if no best block", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)
		ctx := context.Background()

		var calcGenBlk types.Block
		cst := hamt.NewCborStore()
		repo := repo.NewInMemoryRepo()
		chainStore := chain.NewDefaultStore(repo.ChainDatastore(), cst, calcGenBlk.Cid())
		actr := NewActor(chainStore)

		_, _, err := actr.Ls(ctx)
		require.Error(err)
	})
}
