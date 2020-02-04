package actor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	. "github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestActorMarshal(t *testing.T) {
	tf.UnitTest(t)

	actor := NewActor(types.AccountActorCodeCid, types.NewAttoFILFromFIL(1))
	actor.Head = requireCid(t, "Actor Storage")
	actor.IncrementSeqNum()

	marshalled, err := actor.Marshal()
	assert.NoError(t, err)

	actorBack := Actor{}
	err = actorBack.Unmarshal(marshalled)
	assert.NoError(t, err)

	assert.Equal(t, actor.Code, actorBack.Code)
	assert.Equal(t, actor.Head, actorBack.Head)
	assert.Equal(t, actor.CallSeqNum, actorBack.CallSeqNum)

	c1, err := actor.Cid()
	assert.NoError(t, err)
	c2, err := actorBack.Cid()
	assert.NoError(t, err)
	assert.Equal(t, c1, c2)
}

func TestLoadLookup(t *testing.T) {
	tf.UnitTest(t)
	t.Skip("new runtime")

	// ds := datastore.NewMapDatastore()
	// bs := blockstore.NewBlockstore(ds)
	// vms := storagemap.NewStorageMap(bs)
	// storage := vms.NewStorage(address.TestAddress, &Actor{})
	// ctx := context.TODO()

	// lookup, err := LoadLookup(ctx, storage, cid.Undef)
	// require.NoError(t, err)

	// err = lookup.Set(ctx, "foo", "someData")
	// require.NoError(t, err)

	// c, err := lookup.Commit(ctx)
	// require.NoError(t, err)

	// assert.True(t, c.Defined())

	// err = storage.LegacyCommit(c, cid.Undef)
	// require.NoError(t, err)

	// err = vms.Flush()
	// require.NoError(t, err)

	// t.Run("Fetch chunk by cid", func(t *testing.T) {
	// 	bs = blockstore.NewBlockstore(ds)
	// 	vms = storagemap.NewStorageMap(bs)
	// 	storage = vms.NewStorage(address.TestAddress, &Actor{})

	// 	lookup, err = LoadLookup(ctx, storage, c)
	// 	require.NoError(t, err)

	// 	var value string
	// 	err := lookup.Find(ctx, "foo", &value)
	// 	require.NoError(t, err)

	// 	assert.Equal(t, "someData", value)
	// })

	// t.Run("Get errs for missing key", func(t *testing.T) {
	// 	bs = blockstore.NewBlockstore(ds)
	// 	vms = storagemap.NewStorageMap(bs)
	// 	storage = vms.NewStorage(address.TestAddress, &Actor{})

	// 	lookup, err = LoadLookup(ctx, storage, c)
	// 	require.NoError(t, err)

	// 	err := lookup.Find(ctx, "bar", nil)
	// 	require.Error(t, err)
	// 	assert.Equal(t, hamt.ErrNotFound, err)
	// })
}

func TestLoadLookupWithInvalidCid(t *testing.T) {
	tf.UnitTest(t)
	t.Skip("new runtime")

	// ds := datastore.NewMapDatastore()
	// bs := blockstore.NewBlockstore(ds)
	// vms := storagemap.NewStorageMap(bs)
	// storage := vms.NewStorage(address.TestAddress, &Actor{})
	// ctx := context.TODO()

	// c := types.NewCidForTestGetter()()

	// _, err := LoadLookup(ctx, storage, c)
	// require.Error(t, err)
	// assert.Equal(t, storagemap.ErrNotFound, err)
}

func TestSetKeyValue(t *testing.T) {
	tf.UnitTest(t)
	t.Skip("new runtime")

	// ds := datastore.NewMapDatastore()
	// bs := blockstore.NewBlockstore(ds)
	// vms := storagemap.NewStorageMap(bs)
	// storage := vms.NewStorage(address.TestAddress, &Actor{})
	// ctx := context.TODO()

	// c, err := SetKeyValue(ctx, storage, cid.Undef, "foo", "bar")
	// require.NoError(t, err)
	// assert.True(t, c.Defined())

	// lookup, err := LoadLookup(ctx, storage, c)
	// require.NoError(t, err)

	// var val string
	// err = lookup.Find(ctx, "foo", &val)
	// require.NoError(t, err)
	// assert.Equal(t, "bar", val)
}
