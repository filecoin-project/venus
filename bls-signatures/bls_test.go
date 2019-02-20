package bls

import (
	"testing"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
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
	fooMessage := Message("hello foo")
	barMessage := Message("hello bar!")

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

	// assert the bar/foo message was not signed by the foo/bar key
	assert.False(Verify(barSignature, []Digest{barDigest}, []PublicKey{fooPublicKey}))
	assert.False(Verify(barSignature, []Digest{fooDigest}, []PublicKey{barPublicKey}))
	assert.False(Verify(fooSignature, []Digest{barDigest}, []PublicKey{fooPublicKey}))
}
