// These tests check that the signature validation in go-filecoin/types
// works as expected.  They are kept in the wallet package because
// these tests need to generate signatures and the wallet package owns this
// function.  They cannot be kept in types because wallet imports "types"
// for the Signature and KeyInfo types.  TODO: organize packages in a way
// that makes more sense, e.g. so that signature tests can be in same package
// as signature code.

package wallet

import (
	"testing"

	"github.com/ipfs/go-datastore"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/* Test types.IsValidSignature */

func requireSignerAddr(t *testing.T) (*DSBackend, address.Address) {
	ds := datastore.NewMapDatastore()
	fs, err := NewDSBackend(ds)
	require.NoError(t, err)

	addr, err := fs.NewAddress()
	require.NoError(t, err)
	return fs, addr
}

// Signature is over the data being verified and was signed by the verifying
// address.  Everything should work out ok.
func TestSignatureOk(t *testing.T) {
	tf.UnitTest(t)

	fs, addr := requireSignerAddr(t)

	data := []byte("THESE BYTES WILL BE SIGNED")
	sig, err := fs.SignBytes(data, addr)
	require.NoError(t, err)

	assert.True(t, types.IsValidSignature(data, addr, sig))
}

// Signature is nil.
func TestNilSignature(t *testing.T) {
	tf.UnitTest(t)

	_, addr := requireSignerAddr(t)

	data := []byte("THESE BYTES NEED A SIGNATURE")
	assert.False(t, types.IsValidSignature(data, addr, nil))
}

// Signature is over different data.
func TestDataCorrupted(t *testing.T) {
	tf.UnitTest(t)

	fs, addr := requireSignerAddr(t)

	data := []byte("THESE BYTES ARE SIGNED")
	sig, err := fs.SignBytes(data, addr)
	require.NoError(t, err)

	corruptData := []byte("THESE BYTEZ ARE SIGNED")

	assert.False(t, types.IsValidSignature(corruptData, addr, sig))
}

// Signature is valid for data but was signed by a different address.
func TestInvalidAddress(t *testing.T) {
	tf.UnitTest(t)

	fs, addr := requireSignerAddr(t)

	data := []byte("THESE BYTES ARE SIGNED")
	sig, err := fs.SignBytes(data, addr)
	require.NoError(t, err)

	badAddr, err := fs.NewAddress()
	require.NoError(t, err)

	assert.False(t, types.IsValidSignature(data, badAddr, sig))
}

// Signature is corrupted.
func TestSignatureCorrupted(t *testing.T) {
	tf.UnitTest(t)

	fs, addr := requireSignerAddr(t)

	data := []byte("THESE BYTES ARE SIGNED")
	sig, err := fs.SignBytes(data, addr)
	require.NoError(t, err)
	sig[0] = sig[0] ^ 0xFF // This operation ensures sig is modified

	assert.False(t, types.IsValidSignature(data, addr, sig))
}

/* Test types.SignedMessage */

// Valid SignedMessage verifies correctly.
func TestSignMessageOk(t *testing.T) {
	tf.UnitTest(t)

	fs, addr := requireSignerAddr(t)

	msg := types.NewMessage(addr, addr, 1, types.ZeroAttoFIL, "", nil)
	smsg, err := types.NewSignedMessage(*msg, fs, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(t, err)

	assert.True(t, smsg.VerifySignature())
}

// Signature is valid but signer does not match From Address.
func TestBadFrom(t *testing.T) {
	tf.UnitTest(t)

	fs, addr := requireSignerAddr(t)
	addr2, err := fs.NewAddress()
	require.NoError(t, err)

	msg := types.NewMessage(addr, addr, 1, types.ZeroAttoFIL, "", nil)
	meteredMsg := types.NewMeteredMessage(*msg, types.NewGasPrice(0), types.NewGasUnits(0))
	// Can't use NewSignedMessage constructor as it always signs with msg.From.
	bmsg, err := meteredMsg.Marshal()
	require.NoError(t, err)
	sig, err := fs.SignBytes(bmsg, addr2) // sign with addr != msg.From
	require.NoError(t, err)
	smsg := &types.SignedMessage{
		MeteredMessage: *meteredMsg,
		Signature:      sig,
	}

	assert.False(t, smsg.VerifySignature())
}

// Signature corrupted.
func TestSignedMessageBadSignature(t *testing.T) {
	tf.UnitTest(t)

	fs, addr := requireSignerAddr(t)
	msg := types.NewMessage(addr, addr, 1, types.ZeroAttoFIL, "", nil)
	smsg, err := types.NewSignedMessage(*msg, fs, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(t, err)

	smsg.Signature[0] = smsg.Signature[0] ^ 0xFF
	assert.False(t, smsg.VerifySignature())
}

// Message corrupted.
func TestSignedMessageCorrupted(t *testing.T) {
	tf.UnitTest(t)

	fs, addr := requireSignerAddr(t)

	msg := types.NewMessage(addr, addr, 1, types.ZeroAttoFIL, "", nil)
	smsg, err := types.NewSignedMessage(*msg, fs, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(t, err)

	smsg.Message.Nonce = types.Uint64(uint64(42))
	assert.False(t, smsg.VerifySignature())
}
