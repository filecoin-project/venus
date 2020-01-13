package crypto

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"io"

	bls "github.com/filecoin-project/filecoin-ffi"
	secp256k1 "github.com/ipsn/go-secp256k1"
)

// PrivateKeyBytes is the size of a serialized private key.
const PrivateKeyBytes = 32

// PublicKeyBytes is the size of a serialized public key.
const PublicKeyBytes = 65

// PublicKey returns the public key for this private key.
func PublicKey(sk []byte) []byte {
	x, y := secp256k1.S256().ScalarBaseMult(sk)
	return elliptic.Marshal(secp256k1.S256(), x, y)
}

// SignSecp signs the given message using secp256k1 based cryptography, which must be 32 bytes long.
func SignSecp(sk, msg []byte) ([]byte, error) {
	return secp256k1.Sign(msg, sk)
}

// SignBLS signs the given message with BLS.
func SignBLS(sk, msg []byte) ([]byte, error) {
	var privateKey bls.PrivateKey
	copy(privateKey[:], sk)
	sig := bls.PrivateKeySign(privateKey, msg)
	return sig[:], nil
}

// Equals compares two private key for equality and returns true if they are the same.
func Equals(sk, other []byte) bool {
	return bytes.Equal(sk, other)
}

// VerifySecp checks the given signature is a secp256k1 signature and returns true if it is valid.
func VerifySecp(pk, msg, signature []byte) bool {
	if len(signature) == 65 {
		// Drop the V (1byte) in [R | S | V] style signatures.
		// The V (1byte) is the recovery bit and is not apart of the signature verification.
		return secp256k1.VerifySignature(pk[:], msg, signature[:len(signature)-1])
	}

	return secp256k1.VerifySignature(pk[:], msg, signature)
}

// VerifyBLS checks the given signature is valid using BLS cryptography.
func VerifyBLS(pubKey, msg, signature []byte) bool {
	var blsSig bls.Signature
	copy(blsSig[:], signature)
	var blsPubKey bls.PublicKey
	copy(blsPubKey[:], pubKey)
	return bls.Verify(&blsSig, []bls.Digest{bls.Hash(msg)}, []bls.PublicKey{blsPubKey})
}

// VerifyBLSAggregate checks the given signature is a valid aggregate signature over all messages and public keys
func VerifyBLSAggregate(pubKeys, msgs [][]byte, signature []byte) bool {
	digests := []bls.Digest{}
	for _, msg := range msgs {
		digests = append(digests, bls.Hash(msg))
	}

	keys := []bls.PublicKey{}
	for _, pubKey := range pubKeys {
		var blsPubKey bls.PublicKey
		copy(blsPubKey[:], pubKey)
	}

	var blsSig bls.Signature
	copy(blsSig[:], signature)

	return bls.Verify(&blsSig, digests, keys)
}

// GenerateKeyFromSeed generates a new key from the given reader.
func GenerateKeyFromSeed(seed io.Reader) ([]byte, error) {
	key, err := ecdsa.GenerateKey(secp256k1.S256(), seed)
	if err != nil {
		return nil, err
	}

	privkey := make([]byte, PrivateKeyBytes)
	blob := key.D.Bytes()

	// the length is guaranteed to be fixed, given the serialization rules for secp2561k curve points.
	copy(privkey[PrivateKeyBytes-len(blob):], blob)

	return privkey, nil
}

// GenerateKey creates a new key using secure randomness from crypto.rand.
func GenerateKey() ([]byte, error) {
	return GenerateKeyFromSeed(rand.Reader)
}

// EcRecover recovers the public key from a message, signature pair.
func EcRecover(msg, signature []byte) ([]byte, error) {
	return secp256k1.RecoverPubkey(msg, signature)
}
