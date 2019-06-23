package wallet_test

import (
	"bytes"
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"
)

func TestWalletSimple(t *testing.T) {
	tf.UnitTest(t)

	t.Log("create a backend")
	ds := datastore.NewMapDatastore()
	fs, err := wallet.NewDSBackend(ds)
	assert.NoError(t, err)

	t.Log("create a wallet with a single backend")
	w := wallet.New(fs)

	t.Log("check backends")
	assert.Len(t, w.Backends(wallet.DSBackendType), 1)

	t.Log("create a new address in the backend")
	addr, err := fs.NewAddress()
	assert.NoError(t, err)

	t.Log("test HasAddress")
	assert.True(t, w.HasAddress(addr))

	t.Log("find backend")
	backend, err := w.Find(addr)
	assert.NoError(t, err)
	assert.Equal(t, fs, backend)

	t.Log("find unknown address")
	randomAddr := address.NewForTestGetter()()

	assert.False(t, w.HasAddress(randomAddr))

	t.Log("list all addresses")
	list := w.Addresses()
	assert.Len(t, list, 1)
	assert.Equal(t, list[0], addr)

	t.Log("addresses are sorted")
	addr2, err := fs.NewAddress()
	assert.NoError(t, err)

	if bytes.Compare(addr2.Bytes(), addr.Bytes()) < 0 {
		addr, addr2 = addr2, addr
	}
	for i := 0; i < 16; i++ {
		list := w.Addresses()
		assert.Len(t, list, 2)
		assert.Equal(t, list[0], addr)
		assert.Equal(t, list[1], addr2)
	}
}

func TestSimpleSignAndVerify(t *testing.T) {
	tf.UnitTest(t)

	t.Log("create a backend")
	ds := datastore.NewMapDatastore()
	fs, err := wallet.NewDSBackend(ds)
	assert.NoError(t, err)

	t.Log("create a wallet with a single backend")
	w := wallet.New(fs)

	t.Log("check backends")
	assert.Len(t, w.Backends(wallet.DSBackendType), 1)

	t.Log("create a new address in the backend")
	addr, err := fs.NewAddress()
	assert.NoError(t, err)

	t.Log("test HasAddress")
	assert.True(t, w.HasAddress(addr))

	t.Log("find backend")
	backend, err := w.Find(addr)
	assert.NoError(t, err)
	assert.Equal(t, fs, backend)

	// data to sign
	dataA := []byte("THIS IS A SIGNED SLICE OF DATA")
	t.Log("sign content")
	sig, err := w.SignBytes(dataA, addr)
	assert.NoError(t, err)

	// get the key pair for validation
	t.Log("get the key pair from the backend")
	ki, err := backend.GetKeyInfo(addr)
	assert.NoError(t, err)

	pkb := ki.PublicKey()

	t.Log("verify signed content")
	valid, err := w.Verify(dataA, pkb, sig)
	assert.NoError(t, err)
	assert.True(t, valid)

	// data that is unsigned
	dataB := []byte("I AM UNSIGNED DATA!")
	t.Log("verify fails for unsigned content")
	secondValid, err := w.Verify(dataB, pkb, sig)
	assert.NoError(t, err)
	assert.False(t, secondValid)

	t.Log("recovered public key matchs known public key for signed data")
	maybePk, err := w.Ecrecover(dataA, sig)
	assert.NoError(t, err)
	assert.Equal(t, pkb, maybePk)

	t.Log("recovered public key is different than known public key for unsigned data")
	maybePk, err = w.Ecrecover(dataB, sig)
	assert.NoError(t, err)
	assert.NotEqual(t, pkb, maybePk)
}

func TestSignErrorCases(t *testing.T) {
	tf.UnitTest(t)

	t.Log("create 2 backends")
	ds1 := datastore.NewMapDatastore()
	fs1, err := wallet.NewDSBackend(ds1)
	assert.NoError(t, err)

	ds2 := datastore.NewMapDatastore()
	fs2, err := wallet.NewDSBackend(ds2)
	assert.NoError(t, err)

	t.Log("create 2 wallets each with a backend")
	w1 := wallet.New(fs1)
	w2 := wallet.New(fs2)

	t.Log("check backends")
	assert.Len(t, w1.Backends(wallet.DSBackendType), 1)
	assert.Len(t, w2.Backends(wallet.DSBackendType), 1)

	t.Log("create a new address each backend")
	addr1, err := fs1.NewAddress()
	assert.NoError(t, err)
	addr2, err := fs2.NewAddress()
	assert.NoError(t, err)

	t.Log("test HasAddress")
	assert.True(t, w1.HasAddress(addr1))
	assert.False(t, w1.HasAddress(addr2))

	t.Log("find backends")
	backend1, err := w1.Find(addr1)
	assert.NoError(t, err)
	assert.Equal(t, fs1, backend1)

	t.Log("find backend fails for unknown address")
	_, err = w1.Find(addr2)
	assert.Error(t, err)
	assert.Contains(t, wallet.ErrUnknownAddress.Error(), err.Error())

	// data to sign
	dataA := []byte("Set tab width to '1' and make everyone happy")
	t.Log("sign content")
	_, err = w1.SignBytes(dataA, addr2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "could not find address:")
}

func TestGetAddressForPubKeyy(t *testing.T) {
	tf.UnitTest(t)

	ds := datastore.NewMapDatastore()
	fs, err := wallet.NewDSBackend(ds)
	assert.NoError(t, err)
	w := wallet.New(fs)

	for range []int{0, 1, 2} {
		ki, err := w.NewKeyInfo()
		if err != nil {
			panic("w.NewKeyInfo failed for this wallet")
		}

		expectedAddr, _ := ki.Address()
		pubkey := ki.PublicKey()
		actualAddr, err := w.GetAddressForPubKey(pubkey)
		assert.NoError(t, err)
		assert.Equal(t, expectedAddr, actualAddr)
	}

}

func TestWallet_CreateTicket(t *testing.T) {
	tf.UnitTest(t)

	ds := datastore.NewMapDatastore()
	fs, err := wallet.NewDSBackend(ds)
	assert.NoError(t, err)
	w := wallet.New(fs)
	addr, err := wallet.NewAddress(w)
	require.NoError(t, err)

	t.Run("Returns real ticket and nil error with good params", func(t *testing.T) {
		proof := types.PoStProof{0xbb}
		ticket, err := consensus.CreateTicket(proof, addr, w)
		assert.NoError(t, err)
		assert.NotNil(t, ticket)
	})

	t.Run("Returns error and empty ticket when signer is invalid", func(t *testing.T) {
		proof := types.PoStProof{0xc0}
		badAddress := address.TestAddress
		ticket, err := consensus.CreateTicket(proof, badAddress, w)
		assert.Error(t, err, "t, SignBytes error in CreateTicket: public key not found")
		assert.Equal(t, types.Signature(nil), ticket)
	})
}
