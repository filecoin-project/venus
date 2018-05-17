package wallet

import (
	"sync"
	"testing"

	"gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"
	ci "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/types"
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

func TestDSBackendUnmarshalPrivateKey(t *testing.T) {
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

	t.Log("address points to valid secret key")
	bsk, err := ds.Get(datastore.NewKey(addr.String()))
	assert.NoError(err)
	sk, err := ci.UnmarshalPrivateKey(bsk.([]byte))
	assert.NoError(err)

	t.Log("generated address and stored address should match")
	bpk, err := sk.GetPublic().Bytes()
	assert.NoError(err)
	dAdderHash, err := types.AddressHash(bpk)
	assert.NoError(err)
	dAdder := types.NewMainnetAddress(dAdderHash)
	assert.Equal(addr, dAdder)

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
