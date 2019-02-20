package impl

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/xeipuuv/gojsonschema"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestActorLs(t *testing.T) {
	t.Parallel()
	getActorsNoOp := func(st state.Tree) ([]string, []*actor.Actor) {
		return nil, nil
	}

	t.Run("returns an error if no best block", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)
		ctx := context.Background()

		nd := node.MakeOfflineNode(t)

		_, err := ls(ctx, nd, getActorsNoOp)
		require.Error(err)
	})

	t.Run("returns an error if LoadStateTree returns an error", func(t *testing.T) {
		// TOO HARD TO TEST WITHOUT SPECIFIC DEPENDENCY INJECTION
	})

	t.Run("emits json object for each actor in state", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		require := require.New(t)
		ctx := context.Background()

		nd := node.MakeOfflineNode(t)

		genBlock, err := consensus.InitGenesis(nd.CborStore(), nd.Blockstore)
		require.NoError(err)
		b1 := types.NewBlockForTest(genBlock, 1)
		ts := testhelpers.RequireNewTipSet(require, b1)
		chainStore, ok := nd.ChainReader.(chain.Store)
		require.True(ok)

		err = chainStore.PutTipSetAndState(ctx, &chain.TipSetAndState{
			TipSet:          ts,
			TipSetStateRoot: genBlock.StateRoot,
		})
		require.NoError(err)
		err = chainStore.SetHead(ctx, testhelpers.RequireNewTipSet(require, b1))
		require.NoError(err)

		assert.NoError(nd.Start(ctx))
		tokenAmount := types.NewAttoFILFromFIL(100)

		getActors := func(state.Tree) ([]string, []*actor.Actor) {
			actor1, _ := account.NewActor(tokenAmount)
			actor2, _ := storagemarket.NewActor()
			actor3 := miner.NewActor()
			actor4 := actor.NewActor(types.NewCidForTestGetter()(), types.NewAttoFILFromFIL(21))
			return []string{"address1", "address2", "address3", "address4"}, []*actor.Actor{actor1, actor2, actor3, actor4}
		}

		actorViews, err := ls(ctx, nd, getActors)
		require.NoError(err)

		assert.Equal(4, len(actorViews))
		assert.Equal("AccountActor", actorViews[0].ActorType)
		assert.True(tokenAmount.Equal(actorViews[0].Balance))
		assert.Equal("StoragemarketActor", actorViews[1].ActorType)
		assert.Equal("MinerActor", actorViews[2].ActorType)
		assert.Equal("UnknownActor", actorViews[3].ActorType)
	})

	validateActorView := func(a *api.ActorView, sl gojsonschema.JSONLoader) (*gojsonschema.Result, error) {
		jsonBytes, err := json.Marshal(a)
		if err != nil {
			return nil, err
		}
		jsonLoader := gojsonschema.NewBytesLoader(jsonBytes)

		return gojsonschema.Validate(sl, jsonLoader)
	}

	assertSchemaValid := func(t *testing.T, a *api.ActorView, sl gojsonschema.JSONLoader) {
		assert := assert.New(t)
		require := require.New(t)

		result, err := validateActorView(a, sl)
		require.NoError(err)

		assert.True(result.Valid())
		for _, desc := range result.Errors() {
			t.Errorf("- %s\n", desc)
		}
	}

	t.Run("Emitted AccountActor JSON conforms to schema", func(t *testing.T) {
		t.Parallel()

		wd, _ := os.Getwd()
		schemaLoader := gojsonschema.NewReferenceLoader("file://" + wd + "/../../commands/schema/actor_ls.schema.json")

		actor, _ := account.NewActor(types.NewAttoFILFromFIL(100))
		a := makeActorView(actor, "address", &account.Actor{})

		assertSchemaValid(t, a, schemaLoader)

		actor, _ = storagemarket.NewActor()
		head, _ := cid.V1Builder{Codec: cid.DagCBOR, MhType: types.DefaultHashFunction}.Sum([]byte("test cid"))
		actor.Head = head
		a = makeActorView(actor, "address", &storagemarket.Actor{})

		assertSchemaValid(t, a, schemaLoader)

		//addr, _ := types.NewFromString("minerAddress")
		actor = miner.NewActor()
		// addr, []byte{}, types.NewBytesAmount(50000), core.RequireRandomPeerID(), types.NewAttoFILFromFIL(200))
		a = makeActorView(actor, "address", &miner.Actor{})

		assertSchemaValid(t, a, schemaLoader)
	})
}

func TestPresentExports(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	e := (&storagemarket.Actor{}).Exports()
	r := presentExports(e)

	for name, sig := range r {
		s, ok := e[name]
		assert.True(ok)

		for i, x := range sig.Params {
			assert.Equal(s.Params[i].String(), x)
		}
		for i, x := range sig.Return {
			assert.Equal(s.Return[i].String(), x)
		}
	}
}
