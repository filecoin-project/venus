package wallet

import (
	"bytes"
	"context"
	"testing"

	"github.com/filecoin-project/venus/pkg/testhelpers"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-datastore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bls "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/crypto"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func newWalletAndDSBackend(t *testing.T) (*Wallet, *DSBackend) {
	t.Log("create a backend")
	ds := datastore.NewMapDatastore()
	fs, err := NewDSBackend(context.Background(), ds, config.TestPassphraseConfig(), TestPassword)
	assert.NoError(t, err)

	t.Log("create a wallet with a single backend")
	w := New(fs)

	t.Log("check backends")
	assert.Len(t, w.Backends(DSBackendType), 1)

	return w, fs
}

func TestWalletSimple(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	w, fs := newWalletAndDSBackend(t)

	t.Log("create a new address in the backend")
	addr, err := fs.NewAddress(context.Background(), address.SECP256K1)
	assert.NoError(t, err)

	t.Log("test HasAddress")
	assert.True(t, w.HasAddress(ctx, addr))

	t.Log("find backend")
	backend, err := w.Find(ctx, addr)
	assert.NoError(t, err)
	assert.Equal(t, fs, backend)

	t.Log("find unknown address")
	randomAddr := testhelpers.NewForTestGetter()()

	assert.False(t, w.HasAddress(ctx, randomAddr))

	t.Log("list all addresses")
	list := w.Addresses(ctx)
	assert.Len(t, list, 1)
	assert.Equal(t, list[0], addr)

	t.Log("addresses are sorted")
	addr2, err := fs.NewAddress(context.Background(), address.SECP256K1)
	assert.NoError(t, err)

	if bytes.Compare(addr2.Bytes(), addr.Bytes()) < 0 {
		addr, addr2 = addr2, addr
	}
	for i := 0; i < 16; i++ {
		list := w.Addresses(ctx)
		assert.Len(t, list, 2)
		assert.Equal(t, list[0], addr)
		assert.Equal(t, list[1], addr2)
	}
}

func TestWalletBLSKeys(t *testing.T) {
	tf.UnitTest(t)

	w, wb := newWalletAndDSBackend(t)

	ctx := context.Background()
	addr, err := w.NewAddress(ctx, address.BLS)
	require.NoError(t, err)

	data := []byte("data to be signed")
	sig, err := w.SignBytes(ctx, data, addr)
	require.NoError(t, err)

	t.Run("address is BLS protocol", func(t *testing.T) {
		assert.Equal(t, address.BLS, addr.Protocol())
	})

	t.Run("key uses BLS cryptography", func(t *testing.T) {
		ki, err := wb.GetKeyInfo(context.Background(), addr)
		require.NoError(t, err)
		assert.Equal(t, crypto.SigTypeBLS, ki.SigType)
	})

	t.Run("valid signatures verify", func(t *testing.T) {
		err := crypto.Verify(sig, addr, data)
		assert.NoError(t, err)
	})

	t.Run("invalid signatures do not verify", func(t *testing.T) {
		notTheData := []byte("not the data")
		err := crypto.Verify(sig, addr, notTheData)
		assert.Error(t, err)

		notTheSig := &crypto.Signature{
			Type: crypto.SigTypeBLS,
			Data: make([]byte, bls.SignatureBytes),
		}
		copy(notTheSig.Data[:], "not the sig")
		err = crypto.Verify(notTheSig, addr, data)
		assert.Error(t, err)
	})
}

func TestSimpleSignAndVerify(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	w, fs := newWalletAndDSBackend(t)

	t.Log("create a new address in the backend")
	addr, err := fs.NewAddress(context.Background(), address.SECP256K1)
	assert.NoError(t, err)

	t.Log("test HasAddress")
	assert.True(t, w.HasAddress(ctx, addr))

	t.Log("find backend")
	backend, err := w.Find(ctx, addr)
	assert.NoError(t, err)
	assert.Equal(t, fs, backend)

	// data to sign
	dataA := []byte("THIS IS A SIGNED SLICE OF DATA")
	t.Log("sign content")
	sig, err := w.SignBytes(ctx, dataA, addr)
	assert.NoError(t, err)

	t.Log("verify signed content")
	err = crypto.Verify(sig, addr, dataA)
	assert.NoError(t, err)

	// data that is unsigned
	dataB := []byte("I AM UNSIGNED DATA!")
	t.Log("verify fails for unsigned content")
	err = crypto.Verify(sig, addr, dataB)
	assert.Error(t, err)
}

func TestSignErrorCases(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	w1, fs1 := newWalletAndDSBackend(t)
	_, fs2 := newWalletAndDSBackend(t)

	t.Log("create a new address each backend")
	addr1, err := fs1.NewAddress(context.Background(), address.SECP256K1)
	assert.NoError(t, err)
	addr2, err := fs2.NewAddress(context.Background(), address.SECP256K1)
	assert.NoError(t, err)

	t.Log("test HasAddress")
	assert.True(t, w1.HasAddress(ctx, addr1))
	assert.False(t, w1.HasAddress(ctx, addr2))

	t.Log("find backends")
	backend1, err := w1.Find(ctx, addr1)
	assert.NoError(t, err)
	assert.Equal(t, fs1, backend1)

	t.Log("find backend fails for unknown address")
	_, err = w1.Find(ctx, addr2)
	assert.Error(t, err)

	// data to sign
	dataA := []byte("Set tab width to '1' and make everyone happy")
	t.Log("sign content")
	_, err = w1.SignBytes(ctx, dataA, addr2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "could not find address:")
}
