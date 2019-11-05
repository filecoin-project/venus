package storagemap

import (
	"testing"

	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAndPutWithEmptyStorage(t *testing.T) {
	tf.UnitTest(t)

	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	vms := NewStorageMap(bs)
	testActor := actor.NewActor(types.AccountActorCodeCid, types.ZeroAttoFIL)

	t.Run("Put adds to storage", func(t *testing.T) {
		as := vms.NewStorage(address.TestAddress, testActor)

		data, err := cbor.WrapObject("some data an actor might store", types.DefaultHashFunction, -1)
		require.NoError(t, err)

		id, err := as.Put(data.RawData())
		require.NoError(t, err)

		assert.Equal(t, data.Cid(), id)
	})

	t.Run("Get retrieves from storage", func(t *testing.T) {
		as := vms.NewStorage(address.TestAddress, testActor)

		data1, err := encoding.Encode("some data an actor might store")
		require.NoError(t, err)

		data2, err := encoding.Encode("some more data")
		require.NoError(t, err)

		id1, err := as.Put(data1)
		require.NoError(t, err)

		id2, err := as.Put(data2)
		require.NoError(t, err)

		// get both objects from storage
		chunk1, err := as.Get(id1)
		require.NoError(t, err)
		assert.Equal(t, data1, chunk1)

		chunk2, err := as.Get(id2)
		require.NoError(t, err)
		assert.Equal(t, data2, chunk2)
	})

	t.Run("Get is isolated by address", func(t *testing.T) {
		as := vms.NewStorage(address.TestAddress, testActor)

		data1, err := encoding.Encode("some data an actor might store")
		require.NoError(t, err)

		id, err := as.Put(data1)
		require.NoError(t, err)

		// create a storage for another actor
		as2 := vms.NewStorage(address.TestAddress2, testActor)

		// attempt to get from storage
		_, err = as2.Get(id)
		require.Error(t, err)
		assert.Equal(t, ErrNotFound, err)
	})

	t.Run("Storage is consistent when using the same address", func(t *testing.T) {
		as := vms.NewStorage(address.TestAddress, testActor)

		data1, err := encoding.Encode("some data an actor might store")
		require.NoError(t, err)

		id, err := as.Put(data1)
		require.NoError(t, err)

		// create a storage for same actor
		as2 := vms.NewStorage(address.TestAddress, testActor)

		// attempt to get from storage
		chunk, err := as2.Get(id)
		require.NoError(t, err)
		assert.Equal(t, data1, chunk)
	})
}

func TestGetAndPutWithDataInStorage(t *testing.T) {
	tf.UnitTest(t)

	testActor := actor.NewActor(types.AccountActorCodeCid, types.ZeroAttoFIL)

	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	vms := NewStorageMap(bs)

	tempActorStage := vms.NewStorage(address.TestAddress, testActor)
	data1, err := encoding.Encode("some data an actor might store")
	require.NoError(t, err)

	id1, err := tempActorStage.Put(data1)
	require.NoError(t, err)

	data2, err := encoding.Encode("some more data")
	require.NoError(t, err)

	id2, err := tempActorStage.Put(data2)
	require.NoError(t, err)

	t.Run("Get retrieves from storage", func(t *testing.T) {
		as := vms.NewStorage(address.TestAddress, testActor)

		// get both objects from storage
		chunk1, err := as.Get(id1)
		require.NoError(t, err)
		assert.Equal(t, data1, chunk1)

		chunk2, err := as.Get(id2)
		require.NoError(t, err)
		assert.Equal(t, data2, chunk2)
	})

	t.Run("Get is isolated by address", func(t *testing.T) {
		as := vms.NewStorage(address.TestAddress2, testActor) // different address

		// attempt to get from storage
		_, err := as.Get(id1)
		require.Error(t, err)
		assert.Equal(t, ErrNotFound, err)
	})
}

func TestStorageHeadAndCommit(t *testing.T) {
	tf.UnitTest(t)

	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())

	t.Run("Committing changes head", func(t *testing.T) {
		testActor := actor.NewActor(types.AccountActorCodeCid, types.ZeroAttoFIL)

		stage := NewStorageMap(bs).NewStorage(address.TestAddress, testActor)

		newMemory, err := cbor.WrapObject([]byte("New memory"), types.DefaultHashFunction, -1)
		require.NoError(t, err)

		newCid, err := stage.Put(newMemory.RawData())
		require.NoError(t, err)

		assert.NotEqual(t, newCid, stage.Head())

		err = stage.Commit(newCid, stage.Head())
		assert.NoError(t, err)

		assert.Equal(t, newCid, stage.Head())
	})

	t.Run("Committing a non existent chunk is an error", func(t *testing.T) {
		testActor := actor.NewActor(types.AccountActorCodeCid, types.ZeroAttoFIL)

		stage := NewStorageMap(bs).NewStorage(address.TestAddress, testActor)

		newMemory, err := cbor.WrapObject([]byte("New memory"), types.DefaultHashFunction, -1)
		require.NoError(t, err)

		ec := stage.Commit(newMemory.Cid(), stage.Head())
		assert.Equal(t, internal.Errors[internal.ErrDanglingPointer], ec)
	})

	t.Run("Committing out of sequence is an error", func(t *testing.T) {
		testActor := actor.NewActor(types.AccountActorCodeCid, types.ZeroAttoFIL)

		stage := NewStorageMap(bs).NewStorage(address.TestAddress, testActor)

		newMemory1, err := cbor.WrapObject([]byte("New memory 1"), types.DefaultHashFunction, -1)
		require.NoError(t, err)

		_, err = stage.Put(newMemory1.RawData())
		require.NoError(t, err)

		newMemory2, err := cbor.WrapObject([]byte("New memory 2"), types.DefaultHashFunction, -1)
		require.NoError(t, err)

		_, err = stage.Put(newMemory2.RawData())
		require.NoError(t, err)

		err = stage.Commit(newMemory2.Cid(), newMemory1.Cid())
		assert.Equal(t, internal.Errors[internal.ErrStaleHead], err)
	})
}

func TestDatastoreBacking(t *testing.T) {
	tf.UnitTest(t)

	memory2, err := cbor.WrapObject([]byte("Memory chunk 2"), types.DefaultHashFunction, -1)
	require.NoError(t, err)

	memory3, err := cbor.WrapObject([]byte("Memory chunk 3"), types.DefaultHashFunction, -1)
	require.NoError(t, err)

	t.Run("chunks can be retrieved through stage from store", func(t *testing.T) {
		bs := blockstore.NewBlockstore(datastore.NewMapDatastore())

		// add a value to underlying datastore
		require.NoError(t, bs.Put(memory2))

		testActor := actor.NewActor(types.AccountActorCodeCid, types.ZeroAttoFIL)
		stage := NewStorageMap(bs).NewStorage(address.TestAddress, testActor)

		chunk, err := stage.Get(memory2.Cid())
		require.NoError(t, err)
		assert.Equal(t, memory2.RawData(), chunk)
	})

	t.Run("Flush adds chunks to underlying store", func(t *testing.T) {
		bs := blockstore.NewBlockstore(datastore.NewMapDatastore())

		testActor := actor.NewActor(types.AccountActorCodeCid, types.ZeroAttoFIL)
		storage := NewStorageMap(bs)
		stage := storage.NewStorage(address.TestAddress, testActor)

		// put a value into stage
		cid, err := stage.Put(memory2.RawData())
		require.NoError(t, err)

		// commit the change
		assert.NoError(t, stage.Commit(cid, stage.Head()))

		// flush the change
		err = storage.Flush()
		require.NoError(t, err)

		// retrieve cid from underlying store
		chunk, err := bs.Get(cid)
		require.NoError(t, err)
		assert.Equal(t, memory2.RawData(), chunk.RawData())
	})

	t.Run("Flush ignores chunks not referenced through head", func(t *testing.T) {
		bs := blockstore.NewBlockstore(datastore.NewMapDatastore())

		testActor := actor.NewActor(types.AccountActorCodeCid, types.ZeroAttoFIL)
		storage := NewStorageMap(bs)
		stage := storage.NewStorage(address.TestAddress, testActor)

		// put both values into stage
		cid1, err := stage.Put(memory2.RawData())
		require.NoError(t, err)

		cid2, err := stage.Put(memory3.RawData())
		require.NoError(t, err)

		// only commit the second change
		assert.NoError(t, stage.Commit(cid2, stage.Head()))

		// flush the change
		err = storage.Flush()
		require.NoError(t, err)

		// retrieve cid from underlying store
		_, err = bs.Get(cid1)
		assert.Equal(t, blockstore.ErrNotFound, err)

		chunk, err := bs.Get(cid2)
		require.NoError(t, err)
		assert.Equal(t, memory3.RawData(), chunk.RawData())
	})

	t.Run("Flush includes non-head chunks that are referenced in node", func(t *testing.T) {
		bs := blockstore.NewBlockstore(datastore.NewMapDatastore())

		testActor := actor.NewActor(types.AccountActorCodeCid, types.ZeroAttoFIL)
		storage := NewStorageMap(bs)
		stage := storage.NewStorage(address.TestAddress, testActor)

		// put memory 2 into stage
		cid1, err := stage.Put(memory2.RawData())
		require.NoError(t, err)

		// construct a node with memory 2 as a link
		memory4, err := cbor.WrapObject(cid1, types.DefaultHashFunction, -1)
		require.NoError(t, err)

		cid2, err := stage.Put(memory4.RawData())
		require.NoError(t, err)

		// only commit the second change
		assert.NoError(t, stage.Commit(cid2, stage.Head()))

		// flush the change
		err = storage.Flush()
		require.NoError(t, err)

		// retrieve cid from underlying store
		chunk, err := bs.Get(cid1)
		require.NoError(t, err)
		assert.Equal(t, memory2.RawData(), chunk.RawData())
	})
}

func TestValidationAndPruning(t *testing.T) {
	tf.UnitTest(t)

	memory2, err := cbor.WrapObject([]byte("Memory chunk 2"), types.DefaultHashFunction, -1)
	require.NoError(t, err)

	t.Run("Linking to a non-existent cid fails in Commit", func(t *testing.T) {
		bs := blockstore.NewBlockstore(datastore.NewMapDatastore())

		testActor := actor.NewActor(types.AccountActorCodeCid, types.ZeroAttoFIL)
		storage := NewStorageMap(bs)
		stage := storage.NewStorage(address.TestAddress, testActor)

		// This links to memory 2, but memory 2 hasn't been added to anything.
		memory, err := cbor.WrapObject(memory2.Cid(), types.DefaultHashFunction, -1)
		require.NoError(t, err)

		// Add link
		cid, err := stage.Put(memory.RawData())
		assert.NoError(t, err)

		// Attempt to commit before adding linked memory
		err = stage.Commit(cid, stage.Head())
		assert.Equal(t, internal.Errors[internal.ErrDanglingPointer], err)
	})
}
