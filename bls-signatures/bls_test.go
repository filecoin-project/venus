package bls

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBLSSigningAndVerification(t *testing.T) {
	assert := assert.New(t)

	var fooPrivateKeyResponse, _ = PrivateKeyGenerate()
	var fooPrivateKey = fooPrivateKeyResponse.privateKey

	var fooPublicKeyResponse, _ = PrivateKeyPublicKey(PrivateKeyPublicKeyRequest{
		privateKey: fooPrivateKey,
	})
	var fooPublicKey = fooPublicKeyResponse.publicKey

	var fooMessage Message
	copy(fooMessage[:], "hello foo")

	var fooHashResponse, _ = Hash(HashRequest{
		message: fooMessage,
	})
	var fooDigest = fooHashResponse.digest

	var fooSignatureResponse, _ = PrivateKeySign(PrivateKeySignRequest{
		privateKey: fooPrivateKey,
		message:    fooMessage,
	})
	var fooSignature = fooSignatureResponse.signature

	var barPrivateKeyResponse, _ = PrivateKeyGenerate()
	var barPrivateKey = barPrivateKeyResponse.privateKey

	var barPublicKeyResponse, _ = PrivateKeyPublicKey(PrivateKeyPublicKeyRequest{
		privateKey: barPrivateKey,
	})
	var barPublicKey = barPublicKeyResponse.publicKey

	var barMessage Message
	copy(barMessage[:], "hello bar")

	var barHashResponse, _ = Hash(HashRequest{
		message: barMessage,
	})
	var barDigest = barHashResponse.digest

	var barSignatureResponse, _ = PrivateKeySign(PrivateKeySignRequest{
		privateKey: barPrivateKey,
		message:    barMessage,
	})
	var barSignature = barSignatureResponse.signature

	var fooValidVerifyResponse, _ = Verify(VerifyRequest{
		signature:  fooSignature,
		digests:    []Digest{fooDigest},
		publicKeys: []PublicKey{fooPublicKey},
	})
	assert.True(fooValidVerifyResponse.result)

	var barValidVerifyResponse, _ = Verify(VerifyRequest{
		signature:  barSignature,
		digests:    []Digest{barDigest},
		publicKeys: []PublicKey{barPublicKey},
	})
	assert.True(barValidVerifyResponse.result)

	var fooInvalidVerifyResponse, _ = Verify(VerifyRequest{
		signature:  fooSignature,
		digests:    []Digest{fooDigest},
		publicKeys: []PublicKey{barPublicKey},
	})
	assert.False(fooInvalidVerifyResponse.result)

	var barInvalidVerifyResponse, _ = Verify(VerifyRequest{
		signature:  barSignature,
		digests:    []Digest{barDigest},
		publicKeys: []PublicKey{fooPublicKey},
	})
	assert.False(barInvalidVerifyResponse.result)
}
