package bls

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBLSSigningAndVerification(t *testing.T) {
	assert := assert.New(t)

	// generate private keys
	fooPrivateKey := PrivateKeyGenerate()
	barPrivateKey := PrivateKeyGenerate()

	// get the public keys for the private keys
	fooPublicKey := PrivateKeyPublicKey(fooPrivateKey)
	barPublicKey := PrivateKeyPublicKey(barPrivateKey)

	// make messages to sign with the keys
	var fooMessage Message
	copy(fooMessage[:], "hello foo")
	var barMessage Message
	copy(barMessage[:], "hello bar")

	// calculate the digests of the messages
	fooDigest := Hash(fooMessage)
	barDigest := Hash(barMessage)

	// get the signature when signing the messages with the private keys
	fooSignature := PrivateKeySign(fooPrivateKey, fooMessage)
	barSignature := PrivateKeySign(barPrivateKey, barMessage)

	// assert the foo message was signed with the foo key
	assert.True(Verify(fooSignature, []Digest{fooDigest}, []PublicKey{fooPublicKey}))

	// assert the bar message was signed with the bar key
	assert.True(Verify(barSignature, []Digest{barDigest}, []PublicKey{barPublicKey}))

	// assert the foo message was not signed by the bar key
	assert.False(Verify(fooSignature, []Digest{fooDigest}, []PublicKey{barPublicKey}))

	// assert the bar message was not signed by the foo key
	assert.False(Verify(barSignature, []Digest{barDigest}, []PublicKey{fooPublicKey}))
}
