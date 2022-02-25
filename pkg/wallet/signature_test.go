// These tests check that the signature validation in venus/types
// works as expected.  They are kept in the wallet package because
// these tests need to generate signatures and the wallet package owns this
// function.  They cannot be kept in types because wallet imports "types"
// for the Signature and KeyInfo types.  TODO: organize packages in a way
// that makes more sense, e.g. so that signature tests can be in same package
// as signature code.

package wallet

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-datastore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/crypto"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

/* Test types.ValidateSignature */

func requireSignerAddr(t *testing.T) (*DSBackend, address.Address) {
	ds := datastore.NewMapDatastore()
	fs, err := NewDSBackend(context.Background(), ds, config.TestPassphraseConfig(), TestPassword)
	require.NoError(t, err)

	addr, err := fs.NewAddress(context.Background(), address.SECP256K1)
	require.NoError(t, err)

	return fs, addr
}

// Signature is over the data being verified and was signed by the verifying
// address.  Everything should work out ok.
func TestSignatureOk(t *testing.T) {
	tf.UnitTest(t)

	fs, addr := requireSignerAddr(t)

	data := []byte("THESE BYTES WILL BE SIGNED")
	sig, err := fs.SignBytes(context.Background(), data, addr)
	require.NoError(t, err)

	assert.NoError(t, crypto.Verify(sig, addr, data))
}

// Signature is nil.
func TestNilSignature(t *testing.T) {
	tf.UnitTest(t)

	_, addr := requireSignerAddr(t)

	data := []byte("THESE BYTES NEED A SIGNATURE")
	assert.Error(t, crypto.Verify(&crypto.Signature{}, addr, data))
}

// Signature is over different data.
func TestDataCorrupted(t *testing.T) {
	tf.UnitTest(t)

	fs, addr := requireSignerAddr(t)

	data := []byte("THESE BYTES ARE SIGNED")
	sig, err := fs.SignBytes(context.Background(), data, addr)
	require.NoError(t, err)

	corruptData := []byte("THESE BYTEZ ARE SIGNED")

	assert.Error(t, crypto.Verify(sig, addr, corruptData))
}

// Signature is valid for data but was signed by a different address.
func TestInvalidAddress(t *testing.T) {
	tf.UnitTest(t)

	fs, addr := requireSignerAddr(t)

	data := []byte("THESE BYTES ARE SIGNED")
	sig, err := fs.SignBytes(context.Background(), data, addr)
	require.NoError(t, err)

	badAddr, err := fs.NewAddress(context.Background(), address.SECP256K1)
	require.NoError(t, err)

	assert.Error(t, crypto.Verify(sig, badAddr, data))
}

// Signature is corrupted.
func TestSignatureCorrupted(t *testing.T) {
	tf.UnitTest(t)

	fs, addr := requireSignerAddr(t)

	data := []byte("THESE BYTES ARE SIGNED")
	sig, err := fs.SignBytes(context.Background(), data, addr)
	require.NoError(t, err)
	sig.Data[0] = sig.Data[0] ^ 0xFF // This operation ensures sig is modified

	assert.Error(t, crypto.Verify(sig, addr, data))
}
