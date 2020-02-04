package msg

import (
	"context"
	"testing"

	bstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreview(t *testing.T) {
	tf.BadUnitTestWithSideEffects(t)

	t.Run("returns appropriate Gas used", func(t *testing.T) {
		newAddr := address.NewForTestGetter()
		ctx := context.Background()
		r := repo.NewInMemoryRepo()
		bs := bstore.NewBlockstore(r.Datastore())

		fakeActorCodeCid := types.NewCidForTestGetter()()
		fakeActorAddr, err := address.NewIDAddress(1)
		require.NoError(t, err)
		fromAddr := newAddr()
		vms := vm.NewStorageMap(bs)
		fakeActor := th.RequireNewFakeActor(t, vms, fakeActorAddr, fakeActorCodeCid)
		// The genesis init function we give below will install the fake actor at
		// the given address but doesn't set up the mapping from its code cid to
		// actor implementation, so we do that here. Might be nice to handle this
		// setup/teardown through genesis helpers.
		actors := builtin.NewBuilder().
			AddAll(builtin.DefaultActors).
			Add(fakeActorCodeCid, 0, &actor.FakeActor{}).
			Build()
		processor := consensus.NewConfiguredProcessor(consensus.NewDefaultMessageValidator(), actors)
		testGen := consensus.MakeGenesisFunc(
			// Actor we will send the query to.
			consensus.AddActor(fakeActorAddr, fakeActor),
			// Actor we will send the query from. The method we will call returns an Address.
			consensus.ActorAccount(fromAddr, types.NewAttoFILFromFIL(0)),
		)
		deps := requireCommonDepsWithGifAndBlockstore(t, testGen, r, bs)

		previewer := NewPreviewer(deps.chainStore, deps.cst, deps.blockstore, processor)
		returnValue, err := previewer.Preview(ctx, fromAddr, fakeActorAddr, actor.HasReturnValueID)
		require.NoError(t, err)
		require.NotNil(t, returnValue)
		assert.Equal(t, types.NewGasUnits(100), returnValue)
	})
}
