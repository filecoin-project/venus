package wallet

import (
	"testing"

	"gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore"

	"github.com/filecoin-project/go-filecoin/crypto"
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

func TestSimpleSignAndVerify(t *testing.T) {
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

	// data to sign
	dataA := []byte("THIS IS A SIGNED SLICE OF DATA")
	t.Log("sign content")
	sig, err := w.Sign(addr, dataA[:])
	assert.NoError(err)

	// get the key pair for validation
	t.Log("get the key pair from the backend")
	_, pk, err := backend.GetKeyPair(addr)
	assert.NoError(err)
	pkb := crypto.ECDSAPubToBytes(pk)

	t.Log("verify signed content")
	valid, err := w.Verify(pkb, dataA[:], sig)
	assert.NoError(err)
	assert.True(valid)

	// data that is unsigned
	dataB := []byte("I AM UNSIGNED DATA!")
	t.Log("verify fails for unsigned content")
	secondValid, err := w.Verify(pkb, dataB, sig)
	assert.NoError(err)
	assert.False(secondValid)

	t.Log("recovered public key matchs known public key for signed data")
	maybePk, err := w.Ecrecover(dataA, sig)
	assert.NoError(err)
	assert.Equal(pkb, maybePk)

	t.Log("recovered public key is different than known public key for unsigned data")
	maybePk, err = w.Ecrecover(dataB, sig)
	assert.NoError(err)
	assert.NotEqual(pkb, maybePk)
}

func TestSignErrorCases(t *testing.T) {
	assert := assert.New(t)

	t.Log("create 2 backends")
	ds1 := datastore.NewMapDatastore()
	fs1, err := NewDSBackend(ds1)
	assert.NoError(err)

	ds2 := datastore.NewMapDatastore()
	fs2, err := NewDSBackend(ds2)
	assert.NoError(err)

	t.Log("create 2 wallets each with a backend")
	w1 := New(fs1)
	w2 := New(fs2)

	t.Log("check backends")
	assert.Len(w1.Backends(DSBackendType), 1)
	assert.Len(w2.Backends(DSBackendType), 1)

	t.Log("create a new address each backend")
	addr1, err := fs1.NewAddress()
	assert.NoError(err)
	addr2, err := fs2.NewAddress()
	assert.NoError(err)

	t.Log("test HasAddress")
	assert.True(w1.HasAddress(addr1))
	assert.False(w1.HasAddress(addr2))

	t.Log("find backends")
	backend1, err := w1.Find(addr1)
	assert.NoError(err)
	assert.Equal(fs1, backend1)

	t.Log("find backend fails for unknown address")
	_, err = w1.Find(addr2)
	assert.Error(err)
	assert.Contains(ErrUnknownAddress.Error(), err.Error())

	// data to sign
	dataA := []byte("Set tab width to '1' and make everyone happy")
	t.Log("sign content")
	_, err = w1.Sign(addr2, dataA[:])
	assert.Error(err)
	assert.Contains(err.Error(), "failed to sign data")
}
