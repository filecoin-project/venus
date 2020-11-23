package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"
	"io"

	secp256k1 "github.com/ipsn/go-secp256k1"

	bls "github.com/filecoin-project/filecoin-ffi"
)

//
// Abstract SECP and BLS crypto operations.
//

// PrivateKeyBytes is the size of a serialized private key.
const PrivateKeyBytes = 32

////PublicKeyBytes is the size of a serialized public key.
//const PublicKeyBytes = 48

// PublicKeyForSecpSecretKey returns the public key for this private key.
func PublicKeyForSecpSecretKey(sk []byte) []byte {
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
		keys = append(keys, blsPubKey)
	}

	var blsSig bls.Signature
	copy(blsSig[:], signature)
	return bls.Verify(&blsSig, digests, keys)
}

// NewSecpKeyFromSeed generates a new key from the given reader.
func NewSecpKeyFromSeed(seed io.Reader) (KeyInfo, error) {
	key, err := ecdsa.GenerateKey(secp256k1.S256(), seed)
	if err != nil {
		return KeyInfo{}, err
	}

	privkey := make([]byte, PrivateKeyBytes)
	blob := key.D.Bytes()

	// the length is guaranteed to be fixed, given the serialization rules for secp2561k curve points.
	copy(privkey[PrivateKeyBytes-len(blob):], blob)

	return KeyInfo{
		PrivateKey: privkey,
		SigType:    SigTypeSecp256k1,
	}, nil
}

func NewBLSKeyFromSeed(seed io.Reader) (KeyInfo, error) {
	var seedBytes bls.PrivateKeyGenSeed
	read, err := seed.Read(seedBytes[:])
	if err != nil {
		return KeyInfo{}, err
	}
	if read != len(seedBytes) {
		return KeyInfo{}, fmt.Errorf("read only %d bytes of %d required from seed", read, len(seedBytes))
	}
	k := bls.PrivateKeyGenerateWithSeed(seedBytes)
	return KeyInfo{
		PrivateKey: k[:],
		SigType:    SigTypeBLS,
	}, nil
}

// EcRecover recovers the public key from a message, signature pair.
func EcRecover(msg, signature []byte) ([]byte, error) {
	return secp256k1.RecoverPubkey(msg, signature)
}
