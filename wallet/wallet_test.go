package wallet

import (
	"testing"

	"gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
)

func TestWalletSimple(t *testing.T) {
	assert := assert.New(t)

	t.Log("create a backend")
	ds := datastore.NewMapDatastore()
	fs, err := NewDSBackend(ds)
	assert.NoError(err)

	t.Log("create a wallet with a single backend")
	w := New(fs)

	t.Log("check backends")
	assert.Len(w.Backends(DSBackendType), 1)

	t.Log("create a new address in the backend")
	addr, err := fs.NewAddress()
	assert.NoError(err)

	t.Log("test HasAddress")
	assert.True(w.HasAddress(addr))

	t.Log("find backend")
	backend, err := w.Find(addr)
	assert.NoError(err)
	assert.Equal(fs, backend)

	t.Log("find unknown address")
	randomAddr := types.NewAddressForTestGetter()()

	assert.False(w.HasAddress(randomAddr))

	t.Log("list all addresses")
	list := w.Addresses()
	assert.Len(list, 1)
}
