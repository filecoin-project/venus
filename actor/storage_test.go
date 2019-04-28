package actor_test

import (
	"context"
	"math/big"
	"testing"

	cid "github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

func TestActorMarshal(t *testing.T) {
	tf.UnitTest(t)

	actor := NewActor(types.AccountActorCodeCid, types.NewAttoFILFromFIL(1))
	actor.Head = requireCid(t, "Actor Storage")
	actor.IncNonce()

	marshalled, err := actor.Marshal()
	assert.NoError(t, err)

	actorBack := Actor{}
	err = actorBack.Unmarshal(marshalled)
	assert.NoError(t, err)

	assert.Equal(t, actor.Code, actorBack.Code)
	assert.Equal(t, actor.Head, actorBack.Head)
	assert.Equal(t, actor.Nonce, actorBack.Nonce)

	c1, err := actor.Cid()
	assert.NoError(t, err)
	c2, err := actorBack.Cid()
	assert.NoError(t, err)
	assert.Equal(t, c1, c2)
}

func TestMarshalValue(t *testing.T) {
	tf.UnitTest(t)

	t.Run("success", func(t *testing.T) {
		testCases := []struct {
			In  interface{}
			Out []byte
		}{
			{In: []byte("hello"), Out: []byte("hello")},
			{In: big.NewInt(100), Out: big.NewInt(100).Bytes()},
			{In: "hello", Out: []byte("hello")},
		}

		for _, tc := range testCases {
			out, err := MarshalValue(tc.In)
			assert.NoError(t, err)
			assert.Equal(t, out, tc.Out)
		}
	})

	t.Run("failure", func(t *testing.T) {
		out, err := MarshalValue(big.NewRat(1, 2))
		assert.Equal(t, err.Error(), "unknown type: *big.Rat")
		assert.Nil(t, out)
	})
}

func TestLoadLookup(t *testing.T) {
	tf.UnitTest(t)

	ds := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(ds)
	vms := vm.NewStorageMap(bs)
	storage := vms.NewStorage(address.TestAddress, &Actor{})
	ctx := context.TODO()

	lookup, err := LoadLookup(ctx, storage, cid.Undef)
	require.NoError(t, err)

	err = lookup.Set(ctx, "foo", "someData")
	require.NoError(t, err)

	c, err := lookup.Commit(ctx)
	require.NoError(t, err)

	assert.True(t, c.Defined())

	err = storage.Commit(c, cid.Undef)
	require.NoError(t, err)

	err = vms.Flush()
	require.NoError(t, err)

	t.Run("Fetch chunk by cid", func(t *testing.T) {
		bs = blockstore.NewBlockstore(ds)
		vms = vm.NewStorageMap(bs)
		storage = vms.NewStorage(address.TestAddress, &Actor{})

		lookup, err = LoadLookup(ctx, storage, c)
		require.NoError(t, err)

		value, err := lookup.Find(ctx, "foo")
		require.NoError(t, err)

		assert.Equal(t, "someData", value)
	})

	t.Run("Get errs for missing key", func(t *testing.T) {
		bs = blockstore.NewBlockstore(ds)
		vms = vm.NewStorageMap(bs)
		storage = vms.NewStorage(address.TestAddress, &Actor{})

		lookup, err = LoadLookup(ctx, storage, c)
		require.NoError(t, err)

		_, err := lookup.Find(ctx, "bar")
		require.Error(t, err)
		assert.Equal(t, hamt.ErrNotFound, err)
	})
}

func TestLoadLookupWithInvalidCid(t *testing.T) {
	tf.UnitTest(t)

	ds := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(ds)
	vms := vm.NewStorageMap(bs)
	storage := vms.NewStorage(address.TestAddress, &Actor{})
	ctx := context.TODO()

	c := types.NewCidForTestGetter()()

	_, err := LoadLookup(ctx, storage, c)
	require.Error(t, err)
	assert.Equal(t, vm.ErrNotFound, err)
}

func TestSetKeyValue(t *testing.T) {
	tf.UnitTest(t)

	ds := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(ds)
	vms := vm.NewStorageMap(bs)
	storage := vms.NewStorage(address.TestAddress, &Actor{})
	ctx := context.TODO()

	c, err := SetKeyValue(ctx, storage, cid.Undef, "foo", "bar")
	require.NoError(t, err)
	assert.True(t, c.Defined())

	lookup, err := LoadLookup(ctx, storage, c)
	require.NoError(t, err)

	val, err := lookup.Find(ctx, "foo")
	require.NoError(t, err)
	assert.Equal(t, "bar", val)
}
