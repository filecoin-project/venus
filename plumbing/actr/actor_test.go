package actr

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/xeipuuv/gojsonschema"

	"gx/ipfs/QmNf3wujpV2Y7Lnj2hy2UrmuX8bhMDStRHbnSLh7Ypf36h/go-hamt-ipld"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmRu7tiRnFk9mMPpVECQTBQJqXtmG132jJxA1w9A7TtpBz/go-ipfs-blockstore"
)

func TestActorLs(t *testing.T) {
	t.Parallel()

	t.Run("returns an error if no best block", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)
		ctx := context.Background()

		var calcGenBlk types.Block
		cst := hamt.NewCborStore()
		repo := repo.NewInMemoryRepo()
		chainStore := chain.NewDefaultStore(repo.ChainDatastore(), cst, calcGenBlk.Cid())
		actr := NewActor(chainStore)

		_, err := actr.Ls(ctx)
		require.Error(err)
	})

	t.Run("emits json object for each actor in state", func(t *testing.T) {
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
		actorViews, err := actr.Ls(ctx)
		require.NoError(err)

		assert.Equal(5, len(actorViews))
		assert.Equal("StoragemarketActor", actorViews[0].ActorType)
		assert.Equal("AccountActor", actorViews[1].ActorType)
		assert.Equal("AccountActor", actorViews[2].ActorType)
		assert.Equal("AccountActor", actorViews[3].ActorType)
	})
}

func TestMakeActorView(t *testing.T) {
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
}

func assertSchemaValid(t *testing.T, a *ActorView, sl gojsonschema.JSONLoader) {
	assert := assert.New(t)
	require := require.New(t)

	result, err := validateActorView(a, sl)
	require.NoError(err)

	assert.True(result.Valid())
	for _, desc := range result.Errors() {
		t.Errorf("- %s\n", desc)
	}
}

func validateActorView(a *ActorView, sl gojsonschema.JSONLoader) (*gojsonschema.Result, error) {
	jsonBytes, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	jsonLoader := gojsonschema.NewBytesLoader(jsonBytes)

	return gojsonschema.Validate(sl, jsonLoader)
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
