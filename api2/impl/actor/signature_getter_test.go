package actor_test

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
	actor_impl "github.com/filecoin-project/go-filecoin/api2/impl/actor"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
	"github.com/stretchr/testify/require"
)

type fakeChainReadStore struct {
	st state.Tree
}

func (f *fakeChainReadStore) LatestState(ctx context.Context) (state.Tree, error) {
	return f.st, nil
}

func TestGet(t *testing.T) {
	t.Parallel()

	t.Run("succeeds -- note: implementation is fully tested in vm", func(t *testing.T) {
		require := require.New(t)

		ctx := context.Background()
		cst := hamt.NewCborStore()
		addr := address.NewForTestGetter()()
		bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
		vms := vm.NewStorageMap(bs)

		// Install the fake actor so we can query one of its method signatures.
		emptyActorCodeCid := types.NewCidForTestGetter()()
		builtin.Actors[emptyActorCodeCid] = &actor.FakeActor{}
		defer func() {
			delete(builtin.Actors, emptyActorCodeCid)
		}()

		fakeActor := th.RequireNewFakeActorWithTokens(require, vms, addr, emptyActorCodeCid, types.NewAttoFILFromFIL(102))
		_, st := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
			addr: fakeActor,
		})

		getter := actor_impl.NewSignatureGetter(&fakeChainReadStore{st})

		sig, err := getter.Get(ctx, addr, "hasReturnValue")
		require.NoError(err)
		expected := &exec.FunctionSignature{Params: []abi.Type(nil), Return: []abi.Type{abi.Address}}
		require.Equal(expected, sig)
	})
}
