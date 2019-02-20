package actor_test

import (
	"context"
	"math/big"
	"testing"

	"gx/ipfs/QmNf3wujpV2Y7Lnj2hy2UrmuX8bhMDStRHbnSLh7Ypf36h/go-hamt-ipld"
	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmRu7tiRnFk9mMPpVECQTBQJqXtmG132jJxA1w9A7TtpBz/go-ipfs-blockstore"
	"gx/ipfs/QmUadX5EcvrBmxAV9sE7wUWtWSqxns5K84qKJBixmcT1w9/go-datastore"

	. "github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

func TestActorMarshal(t *testing.T) {
	assert := assert.New(t)
	actor := NewActor(types.AccountActorCodeCid, types.NewAttoFILFromFIL(1))
	actor.Head = requireCid(t, "Actor Storage")
	actor.IncNonce()

	marshalled, err := actor.Marshal()
	assert.NoError(err)

	actorBack := Actor{}
	err = actorBack.Unmarshal(marshalled)
	assert.NoError(err)

	assert.Equal(actor.Code, actorBack.Code)
	assert.Equal(actor.Head, actorBack.Head)
	assert.Equal(actor.Nonce, actorBack.Nonce)

	c1, err := actor.Cid()
	assert.NoError(err)
	c2, err := actorBack.Cid()
	assert.NoError(err)
	assert.Equal(c1, c2)
}

func TestMarshalValue(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		assert := assert.New(t)

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
			assert.NoError(err)
			assert.Equal(out, tc.Out)
		}
	})

	t.Run("failure", func(t *testing.T) {
		assert := assert.New(t)

		out, err := MarshalValue(big.NewRat(1, 2))
		assert.Equal(err.Error(), "unknown type: *big.Rat")
		assert.Nil(out)
	})
}

func TestLoadLookup(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	ds := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(ds)
	vms := vm.NewStorageMap(bs)
	storage := vms.NewStorage(address.TestAddress, &Actor{})
	ctx := context.TODO()

	lookup, err := LoadLookup(ctx, storage, cid.Undef)
	require.NoError(err)

	err = lookup.Set(ctx, "foo", "someData")
	require.NoError(err)

	c, err := lookup.Commit(ctx)
	require.NoError(err)

	assert.True(c.Defined())

	err = storage.Commit(c, cid.Undef)
	require.NoError(err)

	err = vms.Flush()
	require.NoError(err)

	t.Run("Fetch chunk by cid", func(t *testing.T) {
		bs = blockstore.NewBlockstore(ds)
		vms = vm.NewStorageMap(bs)
		storage = vms.NewStorage(address.TestAddress, &Actor{})

		lookup, err = LoadLookup(ctx, storage, c)
		require.NoError(err)

		value, err := lookup.Find(ctx, "foo")
		require.NoError(err)

		assert.Equal("someData", value)
	})

	t.Run("Get errs for missing key", func(t *testing.T) {
		bs = blockstore.NewBlockstore(ds)
		vms = vm.NewStorageMap(bs)
		storage = vms.NewStorage(address.TestAddress, &Actor{})

		lookup, err = LoadLookup(ctx, storage, c)
		require.NoError(err)

		_, err := lookup.Find(ctx, "bar")
		require.Error(err)
		assert.Equal(hamt.ErrNotFound, err)
	})
}

func TestLoadLookupWithInvalidCid(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	ds := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(ds)
	vms := vm.NewStorageMap(bs)
	storage := vms.NewStorage(address.TestAddress, &Actor{})
	ctx := context.TODO()

	c := types.NewCidForTestGetter()()

	_, err := LoadLookup(ctx, storage, c)
	require.Error(err)
	assert.Equal(vm.ErrNotFound, err)
}

func TestSetKeyValue(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	ds := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(ds)
	vms := vm.NewStorageMap(bs)
	storage := vms.NewStorage(address.TestAddress, &Actor{})
	ctx := context.TODO()

	c, err := SetKeyValue(ctx, storage, cid.Undef, "foo", "bar")
	require.NoError(err)
	assert.True(c.Defined())

	lookup, err := LoadLookup(ctx, storage, c)
	require.NoError(err)

	val, err := lookup.Find(ctx, "foo")
	require.NoError(err)
	assert.Equal("bar", val)
}
