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

	"gx/ipfs/QmUadX5EcvrBmxAV9sE7wUWtWSqxns5K84qKJBixmcT1w9/go-datastore"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

/* Test types.IsValidSignature */

func requireSignerAddr(require *require.Assertions) (*DSBackend, address.Address) {
	ds := datastore.NewMapDatastore()
	fs, err := NewDSBackend(ds)
	require.NoError(err)

	addr, err := fs.NewAddress()
	require.NoError(err)
	return fs, addr
}

// Signature is over the data being verified and was signed by the verifying
// address.  Everything should work out ok.
func TestSignatureOk(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	fs, addr := requireSignerAddr(require)

	data := []byte("THESE BYTES WILL BE SIGNED")
	sig, err := fs.SignBytes(data, addr)
	require.NoError(err)

	assert.True(types.IsValidSignature(data, addr, sig))
}

// Signature is nil.
func TestNilSignature(t *testing.T) {
	assert := assert.New(t)

	require := require.New(t)
	_, addr := requireSignerAddr(require)

	data := []byte("THESE BYTES NEED A SIGNATURE")
	assert.False(types.IsValidSignature(data, addr, nil))
}

// Signature is over different data.
func TestDataCorrupted(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	fs, addr := requireSignerAddr(require)

	data := []byte("THESE BYTES ARE SIGNED")
	sig, err := fs.SignBytes(data, addr)
	require.NoError(err)

	corruptData := []byte("THESE BYTEZ ARE SIGNED")

	assert.False(types.IsValidSignature(corruptData, addr, sig))
}

// Signature is valid for data but was signed by a different address.
func TestInvalidAddress(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	fs, addr := requireSignerAddr(require)

	data := []byte("THESE BYTES ARE SIGNED")
	sig, err := fs.SignBytes(data, addr)
	require.NoError(err)

	badAddr, err := fs.NewAddress()
	require.NoError(err)

	assert.False(types.IsValidSignature(data, badAddr, sig))
}

// Signature is corrupted.
func TestSignatureCorrupted(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	fs, addr := requireSignerAddr(require)

	data := []byte("THESE BYTES ARE SIGNED")
	sig, err := fs.SignBytes(data, addr)
	require.NoError(err)
	sig[0] = sig[0] ^ 0xFF // This operation ensures sig is modified

	assert.False(types.IsValidSignature(data, addr, sig))
}

/* Test types.SignedMessage */

// Valid SignedMessage verifies correctly.
func TestSignMessageOk(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	fs, addr := requireSignerAddr(require)

	msg := types.NewMessage(addr, addr, 1, nil, "", nil)
	smsg, err := types.NewSignedMessage(*msg, fs, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(err)

	assert.True(smsg.VerifySignature())
}

// Signature is valid but signer does not match From Address.
func TestBadFrom(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	fs, addr := requireSignerAddr(require)
	addr2, err := fs.NewAddress()
	require.NoError(err)

	msg := types.NewMessage(addr, addr, 1, nil, "", nil)
	meteredMsg := types.NewMeteredMessage(*msg, types.NewGasPrice(0), types.NewGasUnits(0))
	// Can't use NewSignedMessage constructor as it always signs with msg.From.
	bmsg, err := meteredMsg.Marshal()
	require.NoError(err)
	sig, err := fs.SignBytes(bmsg, addr2) // sign with addr != msg.From
	require.NoError(err)
	smsg := &types.SignedMessage{
		MeteredMessage: *meteredMsg,
		Signature:      sig,
	}

	assert.False(smsg.VerifySignature())
}

// Signature corrupted.
func TestSignedMessageBadSignature(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	fs, addr := requireSignerAddr(require)
	msg := types.NewMessage(addr, addr, 1, nil, "", nil)
	smsg, err := types.NewSignedMessage(*msg, fs, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(err)

	smsg.Signature[0] = smsg.Signature[0] ^ 0xFF
	assert.False(smsg.VerifySignature())
}

// Message corrupted.
func TestSignedMessageCorrupted(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	fs, addr := requireSignerAddr(require)

	msg := types.NewMessage(addr, addr, 1, nil, "", nil)
	smsg, err := types.NewSignedMessage(*msg, fs, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(err)

	smsg.Message.Nonce = types.Uint64(uint64(42))
	assert.False(smsg.VerifySignature())
}
