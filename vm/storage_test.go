package vm

import (
	"testing"

	cbor "gx/ipfs/QmRiRJhn427YVuufBEHofLreKWNw7P7BWNq86Sb9kzqdbd/go-ipld-cbor"

	"gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAndPutWithEmptyStorage(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	r := repo.NewInMemoryRepo()
	ds := r.Datastore()
	vms := NewStorageMap(ds)
	testActor := types.NewActor(types.AccountActorCodeCid, types.NewZeroAttoFIL())

	t.Run("Put adds to storage", func(t *testing.T) {
		as := vms.NewStorage(address.TestAddress, testActor)

		data, err := cbor.WrapObject("some data an actor might store", types.DefaultHashFunction, -1)
		require.NoError(err)

		id, errorCode := as.Put(data.RawData())
		require.Equal(exec.Ok, errorCode)

		assert.Equal(data.Cid(), id)
	})

	t.Run("Get retrieves from storage", func(t *testing.T) {
		as := vms.NewStorage(address.TestAddress, testActor)

		data1, err := cbor.DumpObject("some data an actor might store")
		require.NoError(err)

		data2, err := cbor.DumpObject("some more data")
		require.NoError(err)

		id1, errorCode := as.Put(data1)
		require.Equal(exec.Ok, errorCode)

		id2, errorCode := as.Put(data2)
		require.Equal(exec.Ok, errorCode)

		// get both objects from storage
		chunk1, ok, err := as.Get(id1)
		require.NoError(err)
		assert.True(ok)
		assert.Equal(data1, chunk1)

		chunk2, ok, err := as.Get(id2)
		require.NoError(err)
		assert.True(ok)
		assert.Equal(data2, chunk2)
	})

	t.Run("Get is isolated by address", func(t *testing.T) {
		as := vms.NewStorage(address.TestAddress, testActor)

		data1, err := cbor.DumpObject("some data an actor might store")
		require.NoError(err)

		id, errorCode := as.Put(data1)
		require.Equal(exec.Ok, errorCode)

		// create a storage for another actor
		as2 := vms.NewStorage(address.TestAddress2, testActor)

		// attempt to get from storage
		_, ok, err := as2.Get(id)
		require.NoError(err)
		assert.False(ok)
	})

	t.Run("Storage is consistent when using the same address", func(t *testing.T) {
		as := vms.NewStorage(address.TestAddress, testActor)

		data1, err := cbor.DumpObject("some data an actor might store")
		require.NoError(err)

		id, errorCode := as.Put(data1)
		require.Equal(exec.Ok, errorCode)

		// create a storage for same actor
		as2 := vms.NewStorage(address.TestAddress, testActor)

		// attempt to get from storage
		chunk, ok, err := as2.Get(id)
		require.NoError(err)
		assert.True(ok)
		assert.Equal(data1, chunk)
	})
}

func TestGetAndPutWithDataInStorage(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	testActor := types.NewActor(types.AccountActorCodeCid, types.NewZeroAttoFIL())

	r := repo.NewInMemoryRepo()
	ds := r.Datastore()
	vms := NewStorageMap(ds)

	tempActorStage := vms.NewStorage(address.TestAddress, testActor)
	data1, err := cbor.DumpObject("some data an actor might store")
	require.NoError(err)

	id1, ec := tempActorStage.Put(data1)
	require.Equal(exec.Ok, ec)

	data2, err := cbor.DumpObject("some more data")
	require.NoError(err)

	id2, ec := tempActorStage.Put(data2)
	require.Equal(exec.Ok, ec)

	t.Run("Get retrieves from storage", func(t *testing.T) {
		as := vms.NewStorage(address.TestAddress, testActor)

		// get both objects from storage
		chunk1, ok, err := as.Get(id1)
		require.NoError(err)
		assert.True(ok)
		assert.Equal(data1, chunk1)

		chunk2, ok, err := as.Get(id2)
		require.NoError(err)
		assert.True(ok)
		assert.Equal(data2, chunk2)
	})

	t.Run("Get is isolated by address", func(t *testing.T) {
		as := vms.NewStorage(address.TestAddress2, testActor) // different address

		// attempt to get from storage
		_, ok, err := as.Get(id1)
		require.NoError(err)
		assert.False(ok)
	})
}

func TestStorageHeadAndCommit(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	startingMemory, err := cbor.WrapObject([]byte("Starting memory"), types.DefaultHashFunction, -1)
	require.NoError(err)

	r := repo.NewInMemoryRepo()
	ds := r.Datastore()

	t.Run("Head of actor matches memory cid", func(t *testing.T) {
		testActor := types.NewActorWithMemory(types.AccountActorCodeCid, types.NewZeroAttoFIL(), startingMemory.RawData())

		stage := NewStorageMap(ds).NewStorage(address.TestAddress, testActor)

		assert.Equal(startingMemory.Cid(), stage.Head())
	})

	t.Run("Committing changes head", func(t *testing.T) {
		testActor := types.NewActorWithMemory(types.AccountActorCodeCid, types.NewZeroAttoFIL(), startingMemory.RawData())

		stage := NewStorageMap(ds).NewStorage(address.TestAddress, testActor)

		newMemory, err := cbor.WrapObject([]byte("New memory"), types.DefaultHashFunction, -1)
		require.NoError(err)

		newCid, ec := stage.Put(newMemory.RawData())
		require.Equal(exec.Ok, ec)

		assert.NotEqual(newCid, stage.Head())

		ec = stage.Commit(newCid, stage.Head())
		assert.Equal(exec.Ok, ec)

		assert.Equal(newCid, stage.Head())
	})

	t.Run("Committing a non existent chunk is an error", func(t *testing.T) {
		testActor := types.NewActorWithMemory(types.AccountActorCodeCid, types.NewZeroAttoFIL(), startingMemory.RawData())

		stage := NewStorageMap(ds).NewStorage(address.TestAddress, testActor)

		newMemory, err := cbor.WrapObject([]byte("New memory"), types.DefaultHashFunction, -1)
		require.NoError(err)

		ec := stage.Commit(newMemory.Cid(), stage.Head())
		assert.Equal(exec.ErrDanglingPointer, ec)
	})

	t.Run("Committing out of sequence is an error", func(t *testing.T) {
		testActor := types.NewActorWithMemory(types.AccountActorCodeCid, types.NewZeroAttoFIL(), startingMemory.RawData())

		stage := NewStorageMap(ds).NewStorage(address.TestAddress, testActor)

		newMemory1, err := cbor.WrapObject([]byte("New memory 1"), types.DefaultHashFunction, -1)
		require.NoError(err)

		_, ec := stage.Put(newMemory1.RawData())
		require.Equal(exec.Ok, ec)

		newMemory2, err := cbor.WrapObject([]byte("New memory 2"), types.DefaultHashFunction, -1)
		require.NoError(err)

		_, ec = stage.Put(newMemory2.RawData())
		require.Equal(exec.Ok, ec)

		ec = stage.Commit(newMemory2.Cid(), newMemory1.Cid())
		assert.Equal(exec.ErrStaleHead, ec)
	})
}

func TestDatastoreBacking(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	startingMemory, err := cbor.WrapObject([]byte("Starting memory"), types.DefaultHashFunction, -1)
	require.NoError(err)

	memory2, err := cbor.WrapObject([]byte("Memory chunk 2"), types.DefaultHashFunction, -1)
	require.NoError(err)

	memory3, err := cbor.WrapObject([]byte("Memory chunk 3"), types.DefaultHashFunction, -1)
	require.NoError(err)

	t.Run("chunks can be retrieved through stage from store", func(t *testing.T) {
		r := repo.NewInMemoryRepo()
		ds := r.Datastore()

		// add a value to underlying datastore
		ds.Put(datastore.NewKey(memory2.Cid().KeyString()), memory2.RawData())

		testActor := types.NewActorWithMemory(types.AccountActorCodeCid, types.NewZeroAttoFIL(), startingMemory.RawData())
		stage := NewStorageMap(ds).NewStorage(address.TestAddress, testActor)

		chunk, ok, err := stage.Get(memory2.Cid())
		require.NoError(err)
		assert.True(ok)
		assert.Equal(memory2.RawData(), chunk)
	})

	t.Run("Flush adds chunks to underlying store", func(t *testing.T) {
		r := repo.NewInMemoryRepo()
		ds := r.Datastore()

		testActor := types.NewActorWithMemory(types.AccountActorCodeCid, types.NewZeroAttoFIL(), startingMemory.RawData())
		storage := NewStorageMap(ds)
		stage := storage.NewStorage(address.TestAddress, testActor)

		// put a value into stage
		cid, ec := stage.Put(memory2.RawData())
		require.Equal(exec.Ok, ec)

		// commit the change
		stage.Commit(cid, stage.Head())

		// flush the change
		err := storage.Flush()
		require.NoError(err)

		// retrieve cid from underlying store
		chunk, err := ds.Get(datastore.NewKey(cid.KeyString()))
		require.NoError(err)
		assert.Equal(memory2.RawData(), chunk)
	})

	t.Run("Flush ignores chunks not referenced through head", func(t *testing.T) {
		r := repo.NewInMemoryRepo()
		ds := r.Datastore()

		testActor := types.NewActorWithMemory(types.AccountActorCodeCid, types.NewZeroAttoFIL(), startingMemory.RawData())
		storage := NewStorageMap(ds)
		stage := storage.NewStorage(address.TestAddress, testActor)

		// put both values into stage
		cid1, ec := stage.Put(memory2.RawData())
		require.Equal(exec.Ok, ec)

		cid2, ec := stage.Put(memory3.RawData())
		require.Equal(exec.Ok, ec)

		// only commit the second change
		stage.Commit(cid2, stage.Head())

		// flush the change
		err := storage.Flush()
		require.NoError(err)

		// retrieve cid from underlying store
		_, err = ds.Get(datastore.NewKey(cid1.KeyString()))
		assert.Equal(datastore.ErrNotFound, err)

		chunk, err := ds.Get(datastore.NewKey(cid2.KeyString()))
		require.NoError(err)
		assert.Equal(memory3.RawData(), chunk)
	})

	t.Run("Flush includes non-head chunks that are referenced in node", func(t *testing.T) {
		r := repo.NewInMemoryRepo()
		ds := r.Datastore()

		testActor := types.NewActorWithMemory(types.AccountActorCodeCid, types.NewZeroAttoFIL(), startingMemory.RawData())
		storage := NewStorageMap(ds)
		stage := storage.NewStorage(address.TestAddress, testActor)

		// put memory 2 into stage
		cid1, ec := stage.Put(memory2.RawData())
		require.Equal(exec.Ok, ec)

		// construct a node with memory 2 as a link
		memory4, err := cbor.WrapObject(cid1, types.DefaultHashFunction, -1)
		require.NoError(err)

		cid2, ec := stage.Put(memory4.RawData())
		require.Equal(exec.Ok, ec)

		// only commit the second change
		stage.Commit(cid2, stage.Head())

		// flush the change
		err = storage.Flush()
		require.NoError(err)

		// retrieve cid from underlying store
		chunk, err := ds.Get(datastore.NewKey(cid1.KeyString()))
		require.NoError(err)
		assert.Equal(memory2.RawData(), chunk)
	})
}

func TestValidationAndPruning(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	startingMemory, err := cbor.WrapObject([]byte("Starting memory"), types.DefaultHashFunction, -1)
	require.NoError(err)

	memory2, err := cbor.WrapObject([]byte("Memory chunk 2"), types.DefaultHashFunction, -1)
	require.NoError(err)

	t.Run("Linking to a non-existent cid fails in Commit", func(t *testing.T) {
		r := repo.NewInMemoryRepo()
		ds := r.Datastore()

		testActor := types.NewActorWithMemory(types.AccountActorCodeCid, types.NewZeroAttoFIL(), startingMemory.RawData())
		storage := NewStorageMap(ds)
		stage := storage.NewStorage(address.TestAddress, testActor)

		// This links to memory 2, but memory 2 hasn't been added to anything.
		memory, err := cbor.WrapObject(memory2.Cid(), types.DefaultHashFunction, -1)
		require.NoError(err)

		// Add link
		cid, ec := stage.Put(memory.RawData())
		assert.Equal(exec.Ok, ec)

		// Attempt to commit before adding linked memory
		ec = stage.Commit(cid, stage.Head())
		assert.Equal(exec.ErrDanglingPointer, ec)
	})

	t.Run("Prune removes unlinked chunks from stage", func(t *testing.T) {
		r := repo.NewInMemoryRepo()
		ds := r.Datastore()

		testActor := types.NewActorWithMemory(types.AccountActorCodeCid, types.NewZeroAttoFIL(), startingMemory.RawData())
		storage := NewStorageMap(ds)
		stage := storage.NewStorage(address.TestAddress, testActor)

		// put both values into stage
		cid1, ec := stage.Put(memory2.RawData())
		require.Equal(exec.Ok, ec)

		memory3, err := cbor.WrapObject([]byte("Memory chunk 3"), types.DefaultHashFunction, -1)
		require.NoError(err)

		cid2, ec := stage.Put(memory3.RawData())
		require.Equal(exec.Ok, ec)

		// only commit the second change
		stage.Commit(cid2, stage.Head())

		// Prune the stage
		err = stage.Prune()
		require.NoError(err)

		// retrieve cid from stage
		_, ok, err := stage.Get(cid1)
		require.NoError(err)
		assert.False(ok)

		chunk, ok, err := stage.Get(cid2)
		require.NoError(err)
		assert.True(ok)
		assert.Equal(memory3.RawData(), chunk)
	})
}
