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

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/ipfs/go-datastore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

/* Test types.ValidateSignature */

func requireSignerAddr(t *testing.T) (*DSBackend, address.Address) {
	ds := datastore.NewMapDatastore()
	fs, err := NewDSBackend(ds)
	require.NoError(t, err)

	addr, err := fs.NewAddress(address.SECP256K1)
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

	assert.NoError(t, crypto.ValidateSignature(data, addr, sig))
}

// Signature is nil.
func TestNilSignature(t *testing.T) {
	tf.UnitTest(t)

	_, addr := requireSignerAddr(t)

	data := []byte("THESE BYTES NEED A SIGNATURE")
	assert.Error(t, crypto.ValidateSignature(data, addr, crypto.Signature{}))
}

// Signature is over different data.
func TestDataCorrupted(t *testing.T) {
	tf.UnitTest(t)

	fs, addr := requireSignerAddr(t)

	data := []byte("THESE BYTES ARE SIGNED")
	sig, err := fs.SignBytes(data, addr)
	require.NoError(t, err)

	corruptData := []byte("THESE BYTEZ ARE SIGNED")

	assert.Error(t, crypto.ValidateSignature(corruptData, addr, sig))
}

// Signature is valid for data but was signed by a different address.
func TestInvalidAddress(t *testing.T) {
	tf.UnitTest(t)

	fs, addr := requireSignerAddr(t)

	data := []byte("THESE BYTES ARE SIGNED")
	sig, err := fs.SignBytes(data, addr)
	require.NoError(t, err)

	badAddr, err := fs.NewAddress(address.SECP256K1)
	require.NoError(t, err)

	assert.Error(t, crypto.ValidateSignature(data, badAddr, sig))
}

// Signature is corrupted.
func TestSignatureCorrupted(t *testing.T) {
	tf.UnitTest(t)

	fs, addr := requireSignerAddr(t)

	data := []byte("THESE BYTES ARE SIGNED")
	sig, err := fs.SignBytes(data, addr)
	require.NoError(t, err)
	sig.Data[0] = sig.Data[0] ^ 0xFF // This operation ensures sig is modified

	assert.Error(t, crypto.ValidateSignature(data, addr, sig))
}

/* Test types.SignedMessage */

// Valid SignedMessage verifies correctly.
func TestSignMessageOk(t *testing.T) {
	tf.UnitTest(t)

	fs, addr := requireSignerAddr(t)

	msg := types.NewMeteredMessage(addr, addr, 1, types.ZeroAttoFIL, builtin.MethodSend, nil, types.NewGasPrice(0), types.GasUnits(0))
	smsg, err := types.NewSignedMessage(*msg, fs)
	require.NoError(t, err)

	assert.NoError(t, smsg.VerifySignature())
}

// Signature is valid but signer does not match From Address.
func TestBadFrom(t *testing.T) {
	tf.UnitTest(t)

	fs, addr := requireSignerAddr(t)
	addr2, err := fs.NewAddress(address.SECP256K1)
	require.NoError(t, err)

	msg := types.NewMeteredMessage(addr, addr, 1, types.ZeroAttoFIL, builtin.MethodSend, nil, types.NewGasPrice(0), types.GasUnits(0))
	// Can't use NewSignedMessage constructor as it always signs with msg.From.
	bmsg, err := msg.Marshal()
	require.NoError(t, err)
	sig, err := fs.SignBytes(bmsg, addr2) // sign with addr != msg.From
	require.NoError(t, err)
	smsg := &types.SignedMessage{
		Message:   *msg,
		Signature: sig,
	}

	assert.Error(t, smsg.VerifySignature())
}

// Signature corrupted.
func TestSignedMessageBadSignature(t *testing.T) {
	tf.UnitTest(t)

	fs, addr := requireSignerAddr(t)
	msg := types.NewMeteredMessage(addr, addr, 1, types.ZeroAttoFIL, builtin.MethodSend, nil, types.NewGasPrice(0), types.GasUnits(0))
	smsg, err := types.NewSignedMessage(*msg, fs)
	require.NoError(t, err)

	smsg.Signature.Data[0] = smsg.Signature.Data[0] ^ 0xFF
	assert.Error(t, smsg.VerifySignature())
}

// Message corrupted.
func TestSignedMessageCorrupted(t *testing.T) {
	tf.UnitTest(t)

	fs, addr := requireSignerAddr(t)

	msg := types.NewMeteredMessage(addr, addr, 1, types.ZeroAttoFIL, builtin.MethodSend, nil, types.NewGasPrice(0), types.GasUnits(0))
	smsg, err := types.NewSignedMessage(*msg, fs)
	require.NoError(t, err)

	smsg.Message.CallSeqNum = uint64(42)
	assert.Error(t, smsg.VerifySignature())
}
