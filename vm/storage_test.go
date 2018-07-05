package vm

import (
	"testing"

	cbor "gx/ipfs/QmRiRJhn427YVuufBEHofLreKWNw7P7BWNq86Sb9kzqdbd/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAndPutWithEmptyStorage(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	vms := StorageMap{}
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
		chunk1, ok := as.Get(id1)
		assert.True(ok)
		assert.Equal(data1, chunk1)

		chunk2, ok := as.Get(id2)
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
		_, ok := as2.Get(id)
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
		chunk, ok := as2.Get(id)
		assert.True(ok)
		assert.Equal(data1, chunk)
	})
}

func TestGetAndPutWithDataInStorage(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	testActor := types.NewActor(types.AccountActorCodeCid, types.NewZeroAttoFIL())

	vms := StorageMap{}
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
		chunk1, ok := as.Get(id1)
		assert.True(ok)
		assert.Equal(data1, chunk1)

		chunk2, ok := as.Get(id2)
		assert.True(ok)
		assert.Equal(data2, chunk2)
	})

	t.Run("Get is isolated by address", func(t *testing.T) {
		as := vms.NewStorage(address.TestAddress2, testActor) // different address

		// attempt to get from storage
		_, ok := as.Get(id1)
		assert.False(ok)
	})
}

func TestStorageHeadAndCommit(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	startingMemory, err := cbor.WrapObject([]byte("Starting memory"), types.DefaultHashFunction, -1)
	require.NoError(err)

	t.Run("Head of actor matches memory cid", func(t *testing.T) {
		testActor := types.NewActorWithMemory(types.AccountActorCodeCid, types.NewZeroAttoFIL(), startingMemory.RawData())

		vms := StorageMap{}
		stage := vms.NewStorage(address.TestAddress, testActor)

		assert.Equal(startingMemory.Cid(), stage.Head())
	})

	t.Run("Committing changes head", func(t *testing.T) {
		testActor := types.NewActorWithMemory(types.AccountActorCodeCid, types.NewZeroAttoFIL(), startingMemory.RawData())

		vms := StorageMap{}
		stage := vms.NewStorage(address.TestAddress, testActor)

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

		vms := StorageMap{}
		stage := vms.NewStorage(address.TestAddress, testActor)

		newMemory, err := cbor.WrapObject([]byte("New memory"), types.DefaultHashFunction, -1)
		require.NoError(err)

		ec := stage.Commit(newMemory.Cid(), stage.Head())
		assert.Equal(exec.ErrDanglingPointer, ec)
	})

	t.Run("Committing out of sequence is an error", func(t *testing.T) {
		testActor := types.NewActorWithMemory(types.AccountActorCodeCid, types.NewZeroAttoFIL(), startingMemory.RawData())

		vms := StorageMap{}
		stage := vms.NewStorage(address.TestAddress, testActor)

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
