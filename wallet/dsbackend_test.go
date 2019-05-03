package wallet

import (
	"sync"
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

func TestDSBackendSimple(t *testing.T) {
	tf.UnitTest(t)

	ds := datastore.NewMapDatastore()
	defer func() {
		require.NoError(t, ds.Close())
	}()

	fs, err := NewDSBackend(ds)
	assert.NoError(t, err)

	t.Log("empty address list on empty datastore")
	assert.Len(t, fs.Addresses(), 0)

	t.Log("can create new address")
	addr, err := fs.NewAddress()
	assert.NoError(t, err)

	t.Log("address is stored")
	assert.True(t, fs.HasAddress(addr))

	t.Log("address is stored in repo, and back when loading fresh in a new backend")
	fs2, err := NewDSBackend(ds)
	assert.NoError(t, err)

	assert.True(t, fs2.HasAddress(addr))
}

func TestDSBackendKeyPairMatchAddress(t *testing.T) {
	tf.UnitTest(t)

	ds := datastore.NewMapDatastore()
	defer func() {
		require.NoError(t, ds.Close())
	}()

	fs, err := NewDSBackend(ds)
	assert.NoError(t, err)

	t.Log("can create new address")
	addr, err := fs.NewAddress()
	assert.NoError(t, err)

	t.Log("address is stored")
	assert.True(t, fs.HasAddress(addr))

	t.Log("address references to a secret key")
	ki, err := fs.GetKeyInfo(addr)
	assert.NoError(t, err)

	dAddr, err := ki.Address()
	assert.NoError(t, err)

	t.Log("generated address and stored address should match")
	assert.Equal(t, addr, dAddr)
}

func TestDSBackendErrorsForUnknownAddress(t *testing.T) {
	tf.UnitTest(t)

	// create 2 backends
	ds1 := datastore.NewMapDatastore()
	defer func() {
		require.NoError(t, ds1.Close())
	}()
	fs1, err := NewDSBackend(ds1)
	assert.NoError(t, err)

	ds2 := datastore.NewMapDatastore()
	defer func() {
		require.NoError(t, ds2.Close())
	}()
	fs2, err := NewDSBackend(ds2)
	assert.NoError(t, err)

	t.Log("can create new address in fs1")
	addr, err := fs1.NewAddress()
	assert.NoError(t, err)

	t.Log("address is stored fs1")
	assert.True(t, fs1.HasAddress(addr))

	t.Log("address is not stored fs2")
	assert.False(t, fs2.HasAddress(addr))

	t.Log("address references to a secret key in fs1")
	_, err = fs1.GetKeyInfo(addr)
	assert.NoError(t, err)

	t.Log("address does not references to a secret key in fs2")
	_, err = fs2.GetKeyInfo(addr)
	assert.Error(t, err)
	assert.Contains(t, "backend does not contain address", err.Error())

}

func TestDSBackendParallel(t *testing.T) {
	tf.UnitTest(t)

	ds := datastore.NewMapDatastore()
	defer func() {
		require.NoError(t, ds.Close())
	}()

	fs, err := NewDSBackend(ds)
	assert.NoError(t, err)

	var wg sync.WaitGroup
	count := 10
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			_, err := fs.NewAddress()
			assert.NoError(t, err)
			wg.Done()
		}()
	}

	wg.Wait()
	assert.Len(t, fs.Addresses(), 10)
}
