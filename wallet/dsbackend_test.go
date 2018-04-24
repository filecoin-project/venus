package wallet

import (
	"sync"
	"testing"

	"gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"

	"github.com/stretchr/testify/assert"
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
