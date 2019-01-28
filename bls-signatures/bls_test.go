package bls

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBLSSigningAndVerification(t *testing.T) {
	assert := assert.New(t)

	// generate private keys
	fooPrivateKeyResponse, _ := PrivateKeyGenerate()
	fooPrivateKey := fooPrivateKeyResponse.privateKey
	barPrivateKeyResponse, _ := PrivateKeyGenerate()
	barPrivateKey := barPrivateKeyResponse.privateKey

	// get the public keys for the private keys
	fooPublicKeyResponse, _ := PrivateKeyPublicKey(PrivateKeyPublicKeyRequest{
		privateKey: fooPrivateKey,
	})
	fooPublicKey := fooPublicKeyResponse.publicKey
	barPublicKeyResponse, _ := PrivateKeyPublicKey(PrivateKeyPublicKeyRequest{
		privateKey: barPrivateKey,
	})
	barPublicKey := barPublicKeyResponse.publicKey

	// make messages to sign with the keys
	var fooMessage Message
	copy(fooMessage[:], "hello foo")
	var barMessage Message
	copy(barMessage[:], "hello bar")

	// calculate the digests of the messages
	fooHashResponse, _ := Hash(HashRequest{
		message: fooMessage,
	})
	fooDigest := fooHashResponse.digest
	barHashResponse, _ := Hash(HashRequest{
		message: barMessage,
	})
	barDigest := barHashResponse.digest

	// get the signature when signing the messages with the private keys
	fooSignatureResponse, _ := PrivateKeySign(PrivateKeySignRequest{
		privateKey: fooPrivateKey,
		message:    fooMessage,
	})
	fooSignature := fooSignatureResponse.signature
	barSignatureResponse, _ := PrivateKeySign(PrivateKeySignRequest{
		privateKey: barPrivateKey,
		message:    barMessage,
	})
	barSignature := barSignatureResponse.signature

	// assert the foo message was signed with the foo key
	fooValidVerifyResponse, _ := Verify(VerifyRequest{
		signature:  fooSignature,
		digests:    []Digest{fooDigest},
		publicKeys: []PublicKey{fooPublicKey},
	})
	assert.True(fooValidVerifyResponse.result)

	// assert the bar message was signed with the bar key
	barValidVerifyResponse, _ := Verify(VerifyRequest{
		signature:  barSignature,
		digests:    []Digest{barDigest},
		publicKeys: []PublicKey{barPublicKey},
	})
	assert.True(barValidVerifyResponse.result)

	// assert the foo message was not signed by the bar key
	fooInvalidVerifyResponse, _ := Verify(VerifyRequest{
		signature:  fooSignature,
		digests:    []Digest{fooDigest},
		publicKeys: []PublicKey{barPublicKey},
	})
	assert.False(fooInvalidVerifyResponse.result)

	// assert the bar message was not signed by the foo key
	barInvalidVerifyResponse, _ := Verify(VerifyRequest{
		signature:  barSignature,
		digests:    []Digest{barDigest},
		publicKeys: []PublicKey{fooPublicKey},
	})
	assert.False(barInvalidVerifyResponse.result)
}
