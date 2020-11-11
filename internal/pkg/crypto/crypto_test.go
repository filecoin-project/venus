package crypto_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bls "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/venus/internal/pkg/crypto"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
)

func TestGenerateSecpKey(t *testing.T) {
	tf.UnitTest(t)

	token := bytes.Repeat([]byte{42}, 512)
	ki, err := crypto.NewSecpKeyFromSeed(bytes.NewReader(token))
	assert.NoError(t, err)
	sk := ki.PrivateKey

	assert.Equal(t, len(sk), 32)

	msg := make([]byte, 32)
	for i := 0; i < len(msg); i++ {
		msg[i] = byte(i)
	}

	digest, err := crypto.SignSecp(sk, msg)
	assert.NoError(t, err)
	assert.Equal(t, len(digest), 65)
	pk := crypto.PublicKeyForSecpSecretKey(sk)

	// valid signature
	assert.True(t, crypto.VerifySecp(pk, msg, digest))

	// invalid signature - different message (too short)
	assert.False(t, crypto.VerifySecp(pk, msg[3:], digest))

	// invalid signature - different message
	msg2 := make([]byte, 32)
	copy(msg2, msg)
	msg2[0] = 42
	assert.False(t, crypto.VerifySecp(pk, msg2, digest))

	// invalid signature - different digest
	digest2 := make([]byte, 65)
	copy(digest2, digest)
	digest2[0] = 42
	assert.False(t, crypto.VerifySecp(pk, msg, digest2))

	// invalid signature - digest too short
	assert.False(t, crypto.VerifySecp(pk, msg, digest[3:]))
	assert.False(t, crypto.VerifySecp(pk, msg, digest[:29]))

	// invalid signature - digest too long
	digest3 := make([]byte, 70)
	copy(digest3, digest)
	assert.False(t, crypto.VerifySecp(pk, msg, digest3))

	recovered, err := crypto.EcRecover(msg, digest)
	assert.NoError(t, err)
	assert.Equal(t, recovered, crypto.PublicKeyForSecpSecretKey(sk))
}

func TestBLSSigning(t *testing.T) {
	privateKey := bls.PrivateKeyGenerate()
	data := []byte("data to be signed")

	signature, err := crypto.SignBLS(privateKey[:], data)
	require.NoError(t, err)

	publicKey := bls.PrivateKeyPublicKey(privateKey)

	valid := crypto.VerifyBLS(publicKey[:], data, signature)
	require.True(t, valid)

	// invalid signature fails
	valid = crypto.VerifyBLS(publicKey[:], data, signature[3:])
	require.False(t, valid)

	// invalid digest fails
	valid = crypto.VerifyBLS(publicKey[:], data[3:], signature)
	require.False(t, valid)

}
