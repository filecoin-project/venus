package vm_test

import (
	"context"
	"testing"

	hamt "gx/ipfs/QmRXf2uUSdGSunRJsM9wXSUNVwLUGCY3So5fAs7h2CBJVf/go-hamt-ipld"
	blockstore "gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"
	datastore "gx/ipfs/Qmf4xQhNomPNhrtZc67qSnfJSjxjXs9LWvknJtSXwimPrM/go-datastore"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSignature(t *testing.T) {
	t.Parallel()

	t.Run("succeeds if method exists", func(t *testing.T) {
		require := require.New(t)

		ctx := context.Background()
		cst := hamt.NewCborStore()
		addr := address.NewForTestGetter()()
		bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
		vms := vm.NewStorageMap(bs)

		// Install the fake actor so we can query one of its method signatures.
		fakeActorCodeCid := types.NewCidForTestGetter()()
		builtin.Actors[fakeActorCodeCid] = &actor.FakeActor{}
		defer func() {
			delete(builtin.Actors, fakeActorCodeCid)
		}()

		fakeActor := th.RequireNewFakeActorWithTokens(require, vms, addr, fakeActorCodeCid, types.NewAttoFILFromFIL(102))
		_, st := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
			addr: fakeActor,
		})

		sig, err := vm.GetSignature(ctx, st, addr, "hasReturnValue")
		require.NoError(err)
		expected := &exec.FunctionSignature{Params: []abi.Type(nil), Return: []abi.Type{abi.Address}}
		require.Equal(expected, sig)
	})

	t.Run("errors if no such method", func(t *testing.T) {
		require := require.New(t)

		ctx := context.Background()
		cst := hamt.NewCborStore()
		addr := address.NewForTestGetter()()

		acctActor := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(10000))
		_, st := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
			addr: acctActor,
		})

		_, err := vm.GetSignature(ctx, st, addr, "NoSuchMethod")
		require.Error(err)
	})

	t.Run("errors with ErrNoMethod if no method", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		ctx := context.Background()
		cst := hamt.NewCborStore()
		addr := address.NewForTestGetter()()

		acctActor := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(10000))
		_, st := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
			addr: acctActor,
		})

		sig, err := vm.GetSignature(ctx, st, addr, "")
		assert.Equal(vm.ErrNoMethod, err)
		assert.Nil(sig)
	})

	t.Run("errors if actor undefined", func(t *testing.T) {
		require := require.New(t)

		ctx := context.Background()
		cst := hamt.NewCborStore()
		addr := address.NewForTestGetter()()

		// Install the fake actor so we can query one of its method signatures.
		fakeActorCodeCid := types.NewCidForTestGetter()()
		builtin.Actors[fakeActorCodeCid] = &actor.FakeActor{}
		defer func() {
			delete(builtin.Actors, fakeActorCodeCid)
		}()
		emptyActor := th.RequireNewEmptyActor(require, types.NewAttoFILFromFIL(0))

		_, st := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
			addr: emptyActor,
		})

		_, err := vm.GetSignature(ctx, st, addr, "")
		require.Error(err)
	})
}
