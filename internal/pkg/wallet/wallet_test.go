package wallet_test

import (
	"bytes"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-datastore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bls "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/venus/internal/pkg/crypto"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
	vmaddr "github.com/filecoin-project/venus/internal/pkg/vm/address"
	"github.com/filecoin-project/venus/internal/pkg/wallet"
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
	addr, err := fs.NewAddress(address.SECP256K1)
	assert.NoError(t, err)

	t.Log("test HasAddress")
	assert.True(t, w.HasAddress(addr))

	t.Log("find backend")
	backend, err := w.Find(addr)
	assert.NoError(t, err)
	assert.Equal(t, fs, backend)

	t.Log("find unknown address")
	randomAddr := vmaddr.NewForTestGetter()()

	assert.False(t, w.HasAddress(randomAddr))

	t.Log("list all addresses")
	list := w.Addresses()
	assert.Len(t, list, 1)
	assert.Equal(t, list[0], addr)

	t.Log("addresses are sorted")
	addr2, err := fs.NewAddress(address.SECP256K1)
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

func TestWalletBLSKeys(t *testing.T) {
	tf.UnitTest(t)

	ds := datastore.NewMapDatastore()
	wb, err := wallet.NewDSBackend(ds)
	require.NoError(t, err)
	w := wallet.New(wb)

	addr, err := wallet.NewAddress(w, address.BLS)
	require.NoError(t, err)

	data := []byte("data to be signed")
	sig, err := w.SignBytes(data, addr)
	require.NoError(t, err)

	t.Run("address is BLS protocol", func(t *testing.T) {
		assert.Equal(t, address.BLS, addr.Protocol())
	})

	t.Run("key uses BLS cryptography", func(t *testing.T) {
		ki, err := wb.GetKeyInfo(addr)
		require.NoError(t, err)
		assert.Equal(t, crypto.SigTypeBLS, ki.SigType)
	})

	t.Run("valid signatures verify", func(t *testing.T) {
		err := crypto.ValidateSignature(data, addr, sig)
		assert.NoError(t, err)
	})

	t.Run("invalid signatures do not verify", func(t *testing.T) {
		notTheData := []byte("not the data")
		err := crypto.ValidateSignature(notTheData, addr, sig)
		assert.Error(t, err)

		notTheSig := crypto.Signature{
			Type: crypto.SigTypeBLS,
			Data: make([]byte, bls.SignatureBytes),
		}
		copy(notTheSig.Data[:], "not the sig")
		err = crypto.ValidateSignature(data, addr, notTheSig)
		assert.Error(t, err)
	})
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
	addr, err := fs.NewAddress(address.SECP256K1)
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

	t.Log("verify signed content")
	err = crypto.ValidateSignature(dataA, addr, sig)
	assert.NoError(t, err)

	// data that is unsigned
	dataB := []byte("I AM UNSIGNED DATA!")
	t.Log("verify fails for unsigned content")
	err = crypto.ValidateSignature(dataB, addr, sig)
	assert.Error(t, err)
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
	addr1, err := fs1.NewAddress(address.SECP256K1)
	assert.NoError(t, err)
	addr2, err := fs2.NewAddress(address.SECP256K1)
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

	// data to sign
	dataA := []byte("Set tab width to '1' and make everyone happy")
	t.Log("sign content")
	_, err = w1.SignBytes(dataA, addr2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "could not find address:")
}
