// stm: #unit
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
	addr, err := fs.NewAddress(ctx, address.SECP256K1)
	assert.NoError(t, err)

	t.Log("test HasAddress")
	// stm: @WALLET_WALLET_HAS_ADDRESS_001
	assert.True(t, w.HasAddress(ctx, addr))

	t.Log("find backend")
	// stm: @WALLET_WALLET_FIND_001
	backend, err := w.Find(ctx, addr)
	assert.NoError(t, err)
	assert.Equal(t, fs, backend)

	t.Log("find unknown address")
	randomAddr := testhelpers.NewForTestGetter()()

	assert.False(t, w.HasAddress(ctx, randomAddr))

	t.Log("list all addresses")
	// stm: @WALLET_WALLET_ADDRESSES_001
	list := w.Addresses(ctx)
	assert.Len(t, list, 1)
	assert.Equal(t, list[0], addr)

	t.Log("addresses are sorted")
	addr2, err := fs.NewAddress(ctx, address.SECP256K1)
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

	t.Log("test DeleteAddress")
	err = fs.DeleteAddress(ctx, addr)
	assert.NoError(t, err)
	err = fs.DeleteAddress(ctx, addr2)
	assert.NoError(t, err)
	assert.False(t, w.HasAddress(ctx, addr))
	assert.False(t, w.HasAddress(ctx, addr2))
}

func TestWalletBLSKeys(t *testing.T) {
	tf.UnitTest(t)

	w, wb := newWalletAndDSBackend(t)

	ctx := context.Background()
	addr, err := w.NewAddress(ctx, address.BLS)
	require.NoError(t, err)

	data := []byte("data to be signed")
	// stm: @WALLET_WALLET_SIGN_BYTES_001
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
	// stm: @WALLET_WALLET_NEW_ADDRESS_001
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

func TestImportExport(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	password := "test password"
	TestPassword = []byte(password)
	w1, _ := newWalletAndDSBackend(t)
	TestPassword = []byte(password)
	w2, _ := newWalletAndDSBackend(t)
	// stm: @WALLET_WALLET_NEW_ADDRESS_001
	addr1, err := w1.NewAddress(ctx, address.BLS)
	assert.NoError(t, err)
	// stm: @WALLET_WALLET_GET_ADDRESS_PUBKEY_001
	pubKey, err := w1.GetPubKeyForAddress(ctx, addr1)
	assert.NoError(t, err)
	addr2, err := address.NewBLSAddress(pubKey)
	assert.NoError(t, err)
	assert.Equal(t, addr1, addr2)
	// stm: @WALLET_WALLET_EXPORT_001
	keyInfo, err := w1.Export(ctx, addr1, password)
	assert.NoError(t, err)
	// stm: @WALLET_WALLET_IMPORT_001
	addr2, err = w2.Import(ctx, keyInfo)
	assert.NoError(t, err)
	assert.Equal(t, addr1, addr2)
	// stm: @WALLET_WALLET_NEW_KEY_INFO_001
	keyInfo, err = w2.NewKeyInfo(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, keyInfo)
	// stm: @WALLET_WALLET_HAS_ADDRESS_001
	assert.True(t, w2.HasAddress(ctx, addr1))
	addr2, err = keyInfo.Address()
	assert.NoError(t, err)
	assert.True(t, w2.HasAddress(ctx, addr2))
	// stm: @WALLET_WALLET_HAS_PASSWORD_001
	assert.True(t, w2.HasPassword(ctx))
	// stm: @WALLET_WALLET_LOCK_001
	assert.NoError(t, w2.LockWallet(ctx))
	// stm: @WALLET_WALLET_GET_STATE_001
	assert.Equal(t, w2.WalletState(ctx), Lock)
	// stm: @WALLET_WALLET_UNLOCK_001
	assert.Error(t, w2.UnLockWallet(ctx, []byte("wrong password")))
	assert.NoError(t, w2.UnLockWallet(ctx, []byte(password)))
	// stm: @WALLET_WALLET_GET_STATE_001
	assert.Equal(t, w2.WalletState(ctx), Unlock)
}
