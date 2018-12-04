package wallet

import (
	"sync"
	"testing"

	"gx/ipfs/Qmf4xQhNomPNhrtZc67qSnfJSjxjXs9LWvknJtSXwimPrM/go-datastore"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDSBackendSimple(t *testing.T) {
	assert := assert.New(t)

	ds := datastore.NewMapDatastore()
	defer ds.Close()

	fs, err := NewDSBackend(ds)
	assert.NoError(err)

	t.Log("empty address list on empty datastore")
	assert.Len(fs.Addresses(), 0)

	t.Log("can create new address")
	addr, err := fs.NewAddress()
	assert.NoError(err)

	t.Log("address is stored")
	assert.True(fs.HasAddress(addr))

	t.Log("address is stored in repo, and back when loading fresh in a new backend")
	fs2, err := NewDSBackend(ds)
	assert.NoError(err)

	assert.True(fs2.HasAddress(addr))
}

func TestDSBackendKeyPairMatchAddress(t *testing.T) {
	assert := assert.New(t)

	ds := datastore.NewMapDatastore()
	defer ds.Close()

	fs, err := NewDSBackend(ds)
	assert.NoError(err)

	t.Log("can create new address")
	addr, err := fs.NewAddress()
	assert.NoError(err)

	t.Log("address is stored")
	assert.True(fs.HasAddress(addr))

	t.Log("address references to a secret key")
	ki, err := fs.GetKeyInfo(addr)
	assert.NoError(err)

	dAddr, err := ki.Address()
	assert.NoError(err)

	t.Log("generated address and stored address should match")
	assert.Equal(addr, dAddr)
}

func TestDSBackendErrorsForUnknownAddress(t *testing.T) {
	assert := assert.New(t)

	// create 2 backends
	ds1 := datastore.NewMapDatastore()
	defer ds1.Close()
	fs1, err := NewDSBackend(ds1)
	assert.NoError(err)

	ds2 := datastore.NewMapDatastore()
	defer ds2.Close()
	fs2, err := NewDSBackend(ds2)
	assert.NoError(err)

	t.Log("can create new address in fs1")
	addr, err := fs1.NewAddress()
	assert.NoError(err)

	t.Log("address is stored fs1")
	assert.True(fs1.HasAddress(addr))

	t.Log("address is not stored fs2")
	assert.False(fs2.HasAddress(addr))

	t.Log("address references to a secret key in fs1")
	_, err = fs1.GetKeyInfo(addr)
	assert.NoError(err)

	t.Log("address does not references to a secret key in fs2")
	_, err = fs2.GetKeyInfo(addr)
	assert.Error(err)
	assert.Contains("backend does not contain address", err.Error())

}

func TestDSBackendLoadAddressInfo(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ds := datastore.NewMapDatastore()
	defer ds.Close()

	fs, err := NewDSBackend(ds)
	assert.NoError(err)

	t.Log("empty address list on empty datastore")
	assert.Len(fs.Addresses(), 0)

	t.Log("generate a WalletFile with 2 addresses")
	wf, err := GenerateWalletFile(2)
	require.NoError(err)

	t.Log("load address into datastore")
	assert.NoError(fs.ImportKey(&wf.AddressKeyPairs[0].KeyInfo))

	t.Log("datastore has 1 address")
	assert.Len(fs.Addresses(), 1)

	t.Log("load address info into datastore")
	assert.NoError(fs.ImportKey(&wf.AddressKeyPairs[1].KeyInfo))

	t.Log("datastore has 2 address")
	assert.Len(fs.Addresses(), 2)
}

func TestDSBackendParallel(t *testing.T) {
	assert := assert.New(t)

	ds := datastore.NewMapDatastore()
	defer ds.Close()

	fs, err := NewDSBackend(ds)
	assert.NoError(err)

	var wg sync.WaitGroup
	count := 10
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			_, err := fs.NewAddress()
			assert.NoError(err)
			wg.Done()
		}()
	}

	wg.Wait()
	assert.Len(fs.Addresses(), 10)
}
