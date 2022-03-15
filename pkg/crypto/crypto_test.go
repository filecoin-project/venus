package crypto_test

import (
	"bytes"
	"testing"

	"github.com/filecoin-project/go-address"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bls "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/venus/pkg/crypto"
	_ "github.com/filecoin-project/venus/pkg/crypto/bls"
	_ "github.com/filecoin-project/venus/pkg/crypto/secp"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func TestGenerateSecpKey(t *testing.T) {
	tf.UnitTest(t)

	token := bytes.Repeat([]byte{42}, 512)
	ki, err := crypto.NewSecpKeyFromSeed(bytes.NewReader(token))
	assert.NoError(t, err)
	sk := ki.Key()
	t.Logf("%x", sk)
	assert.Equal(t, len(sk), 32)

	msg := make([]byte, 32)
	for i := 0; i < len(msg); i++ {
		msg[i] = byte(i)
	}

	signature, err := crypto.Sign(msg, sk, crypto.SigTypeSecp256k1)
	assert.NoError(t, err)
	assert.Equal(t, len(signature.Data), 65)
	pk, err := crypto.ToPublic(crypto.SigTypeSecp256k1, sk)
	assert.NoError(t, err)
	addr, err := address.NewSecp256k1Address(pk)
	assert.NoError(t, err)
	t.Logf("%x", pk)
	// valid signature
	assert.True(t, crypto.Verify(signature, addr, msg) == nil)

	// invalid signature - different message (too short)
	assert.False(t, crypto.Verify(signature, addr, msg[3:]) == nil)

	// invalid signature - different message
	msg2 := make([]byte, 32)
	copy(msg2, msg)
	msg2[0] = 42
	assert.False(t, crypto.Verify(signature, addr, msg2) == nil)

	// invalid signature - different digest
	digest2 := make([]byte, 65)
	copy(digest2, signature.Data)
	digest2[0] = 42
	assert.False(t, crypto.Verify(&crypto.Signature{Type: crypto.SigTypeSecp256k1, Data: digest2}, addr, msg) == nil)

	// invalid signature - digest too short
	assert.False(t, crypto.Verify(&crypto.Signature{Type: crypto.SigTypeSecp256k1, Data: signature.Data[3:]}, addr, msg) == nil)
	assert.False(t, crypto.Verify(&crypto.Signature{Type: crypto.SigTypeSecp256k1, Data: signature.Data[:29]}, addr, msg) == nil)

	// invalid signature - digest too long
	digest3 := make([]byte, 70)
	copy(digest3, signature.Data)
	assert.False(t, crypto.Verify(&crypto.Signature{Type: crypto.SigTypeSecp256k1, Data: digest3}, addr, msg) == nil)
}

func TestBLSSigning(t *testing.T) {
	privateKey := bls.PrivateKeyGenerate()
	data := []byte("data to be signed")
	t.Logf("%x", privateKey)
	t.Logf("%x", bls.PrivateKeyPublicKey(privateKey))
	signature, err := crypto.Sign(data, privateKey[:], crypto.SigTypeBLS)
	require.NoError(t, err)

	publicKey := bls.PrivateKeyPublicKey(privateKey)
	addr, err := address.NewBLSAddress(publicKey[:])
	require.NoError(t, err)

	err = crypto.Verify(signature, addr, data)
	require.NoError(t, err)

	// invalid signature fails
	err = crypto.Verify(&crypto.Signature{Type: crypto.SigTypeBLS, Data: signature.Data[3:]}, addr, data)
	require.Error(t, err)

	// invalid digest fails
	err = crypto.Verify(signature, addr, data[3:])
	require.Error(t, err)

}
