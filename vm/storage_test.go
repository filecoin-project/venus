package vm

import (
	"testing"

	cbor "gx/ipfs/QmRiRJhn427YVuufBEHofLreKWNw7P7BWNq86Sb9kzqdbd/go-ipld-cbor"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAndPutWithEmptyStorage(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	vms := Storage{}

	t.Run("Put adds to storage", func(t *testing.T) {
		as := vms.NewStage(address.TestAddress)

		data, err := cbor.DumpObject("some data an actor might store")
		require.NoError(err)

		id, errorCode := as.Put(data)
		require.Equal(exec.Ok, errorCode)

		expectedID := cborDecodeCid(data, t)
		assert.Equal(expectedID, id)
	})

	t.Run("Get retrieves from storage", func(t *testing.T) {
		as := vms.NewStage(address.TestAddress)

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
		as := vms.NewStage(address.TestAddress)

		data1, err := cbor.DumpObject("some data an actor might store")
		require.NoError(err)

		id, errorCode := as.Put(data1)
		require.Equal(exec.Ok, errorCode)

		// create a storage for another actor
		as2 := vms.NewStage(address.TestAddress2)

		// attempt to get from storage
		_, ok := as2.Get(id)
		assert.False(ok)
	})

	t.Run("NewStage is consistent when using the same address", func(t *testing.T) {
		as := vms.NewStage(address.TestAddress)

		data1, err := cbor.DumpObject("some data an actor might store")
		require.NoError(err)

		id, errorCode := as.Put(data1)
		require.Equal(exec.Ok, errorCode)

		// create a storage for same actor
		as2 := vms.NewStage(address.TestAddress)

		// attempt to get from storage
		chunk, ok := as2.Get(id)
		assert.True(ok)
		assert.Equal(data1, chunk)
	})
}

func TestGetAndPutWithDataInStorage(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	vms := Storage{}
	tempActorStage := vms.NewStage(address.TestAddress)
	data1, err := cbor.DumpObject("some data an actor might store")
	require.NoError(err)

	id1, ec := tempActorStage.Put(data1)
	require.Equal(exec.Ok, ec)

	data2, err := cbor.DumpObject("some more data")
	require.NoError(err)

	id2, ec := tempActorStage.Put(data2)
	require.Equal(exec.Ok, ec)

	t.Run("Get retrieves from storage", func(t *testing.T) {
		as := vms.NewStage(address.TestAddress)

		// get both objects from storage
		chunk1, ok := as.Get(id1)
		assert.True(ok)
		assert.Equal(data1, chunk1)

		chunk2, ok := as.Get(id2)
		assert.True(ok)
		assert.Equal(data2, chunk2)
	})

	t.Run("Get is isolated by address", func(t *testing.T) {
		as := vms.NewStage(address.TestAddress2) // different address

		// attempt to get from storage
		_, ok := as.Get(id1)
		assert.False(ok)
	})
}

func cborDecodeCid(chunk []byte, t *testing.T) *cid.Cid {
	n, err := cbor.Decode(chunk, types.DefaultHashFunction, -1)
	require.NoError(t, err)

	return n.Cid()
}
