package consensus_test

import (
	"context"
	"testing"

	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	. "github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

func TestQuery(t *testing.T) {
	tf.BadUnitTestWithSideEffects(t)

	t.Run("success", func(t *testing.T) {
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
		// setup/teardown through geneisus helpers.

		actors := builtin.NewBuilder().AddAll(builtin.DefaultActors).Add(fakeActorCodeCid, 0, &actor.FakeActor{}).Build()
		processor := NewConfiguredProcessor(NewDefaultMessageValidator(), actors)
		testGen := MakeGenesisFunc(
			// Actor we will send the query to.
			AddActor(fakeActorAddr, fakeActor),
			// Actor we will send the query from. The method we will call returns an Address.
			ActorAccount(fromAddr, types.NewAttoFILFromFIL(0)),
		)
		cst := hamt.CSTFromBstore(bs)
		chainStore, err := chain.Init(context.Background(), r, bs, cst, testGen)
		require.NoError(t, err)

		chainState := NewActorStateStore(chainStore, cst, bs, processor)
		snapshot, err := chainState.Snapshot(ctx, chainStore.GetHead())
		require.NoError(t, err)

		returnValue, err := snapshot.Query(ctx, fromAddr, fakeActorAddr, actor.HasReturnValueID)
		require.NoError(t, err)
		require.NotNil(t, returnValue)
		v, err := abi.Deserialize(returnValue[0], abi.Address)
		require.NoError(t, err)
		_, ok := v.Val.(address.Address)
		require.True(t, ok)
	})

	t.Run("non-zero exit code is an error", func(t *testing.T) {
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
		// setup/teardown through geneisus helpers.
		actors := builtin.NewBuilder().
			AddAll(builtin.DefaultActors).
			Add(fakeActorCodeCid, 0, &actor.FakeActor{}).
			Build()
		processor := NewConfiguredProcessor(NewDefaultMessageValidator(), actors)
		testGen := MakeGenesisFunc(
			// Actor we will send the query to.
			AddActor(fakeActorAddr, fakeActor),
			// Actor we will send the query from. The method we will call returns an Address.
			ActorAccount(fromAddr, types.NewAttoFILFromFIL(0)),
		)
		cst := hamt.CSTFromBstore(bs)
		chainStore, err := chain.Init(context.Background(), r, bs, cst, testGen)
		require.NoError(t, err)

		chainState := NewActorStateStore(chainStore, cst, bs, processor)
		snapshot, err := chainState.Snapshot(ctx, chainStore.GetHead())
		require.NoError(t, err)

		_, err = snapshot.Query(ctx, fromAddr, fakeActorAddr, actor.NonZeroExitCodeID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "42")
	})
}
