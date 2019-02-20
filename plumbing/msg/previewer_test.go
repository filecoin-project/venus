package msg

import (
	"context"
	"testing"

	bstore "gx/ipfs/QmRu7tiRnFk9mMPpVECQTBQJqXtmG132jJxA1w9A7TtpBz/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/repo"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

func TestPreview(t *testing.T) {
	// Don't add t.Parallel here; these tests muck with globals.

	t.Run("returns appropriate Gas used", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)
		newAddr := address.NewForTestGetter()
		ctx := context.Background()
		r := repo.NewInMemoryRepo()
		bs := bstore.NewBlockstore(r.Datastore())

		fakeActorCodeCid := types.NewCidForTestGetter()()
		fakeActorAddr := newAddr()
		fromAddr := newAddr()
		vms := vm.NewStorageMap(bs)
		fakeActor := th.RequireNewFakeActor(require, vms, fakeActorAddr, fakeActorCodeCid)
		// The genesis init function we give below will install the fake actor at
		// the given address but doesn't set up the mapping from its code cid to
		// actor implementation, so we do that here. Might be nice to handle this
		// setup/teardown through genesis helpers.
		builtin.Actors[fakeActorCodeCid] = &actor.FakeActor{}
		defer delete(builtin.Actors, fakeActorCodeCid)
		testGen := consensus.MakeGenesisFunc(
			// Actor we will send the query to.
			consensus.AddActor(fakeActorAddr, fakeActor),
			// Actor we will send the query from. The method we will call returns an Address.
			consensus.ActorAccount(fromAddr, types.NewAttoFILFromFIL(0)),
		)
		deps := requireCommonDepsWithGifAndBlockstore(require, testGen, r, bs)

		previewer := NewPreviewer(deps.wallet, deps.chainStore, deps.cst, deps.blockstore)
		returnValue, err := previewer.Preview(ctx, fromAddr, fakeActorAddr, "hasReturnValue")
		require.NoError(err)
		require.NotNil(returnValue)
		assert.Equal(types.NewGasUnits(100), returnValue)
	})
}
