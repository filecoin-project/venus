package bls

import (
	"crypto/rand"
	"fmt"
	"io"

	crypto2 "github.com/filecoin-project/venus/pkg/crypto"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/crypto"

	ffi "github.com/filecoin-project/filecoin-ffi"
)

const DST = string("BLS_SIG_BLS12381G2_XMD:SHA-256_SSWU_RO_NUL_")

type SecretKey = ffi.PrivateKey
type PublicKey = ffi.PublicKey
type Signature = ffi.Signature
type AggregateSignature = ffi.Signature

type blsSigner struct{}

func (s blsSigner) VerifyAggregate(pubKeys, msgs [][]byte, signature []byte) bool {
	digests := []ffi.Digest{}
	for _, msg := range msgs {
		digests = append(digests, ffi.Hash(msg))
	}

	keys := []ffi.PublicKey{}
	for _, pubKey := range pubKeys {
		var blsPubKey ffi.PublicKey
		copy(blsPubKey[:], pubKey)
		keys = append(keys, blsPubKey)
	}

	var blsSig ffi.Signature
	copy(blsSig[:], signature)
	return ffi.Verify(&blsSig, digests, keys)
}

func (blsSigner) GenPrivate() ([]byte, error) {
	// Generate 32 bytes of randomness
	var ikm [32]byte
	_, err := rand.Read(ikm[:])
	if err != nil {
		return nil, fmt.Errorf("bls signature error generating random data")
	}
	// Note private keys seem to be serialized little-endian!
	sk := ffi.PrivateKeyGenerateWithSeed(ikm)
	return sk[:], nil
}

func (blsSigner) GenPrivateFromSeed(seed io.Reader) ([]byte, error) {
	var seedBytes ffi.PrivateKeyGenSeed
	read, err := seed.Read(seedBytes[:])
	if err != nil {
		return nil, err
	}
	if read != len(seedBytes) {
		return nil, fmt.Errorf("read only %d bytes of %d required from seed", read, len(seedBytes))
	}
	priKey := ffi.PrivateKeyGenerateWithSeed(seedBytes)
	return priKey[:], nil
}

func (blsSigner) ToPublic(priv []byte) ([]byte, error) {
	if priv == nil || len(priv) != ffi.PrivateKeyBytes {
		return nil, fmt.Errorf("bls signature invalid private key")
	}

	sk := new(SecretKey)
	copy(sk[:], priv[:ffi.PrivateKeyBytes])

	pubkey := ffi.PrivateKeyPublicKey(*sk)

	return pubkey[:], nil
}

func (blsSigner) Sign(p []byte, msg []byte) ([]byte, error) {
	if p == nil || len(p) != ffi.PrivateKeyBytes {
		return nil, fmt.Errorf("bls signature invalid private key")
	}

	sk := new(SecretKey)
	copy(sk[:], p[:ffi.PrivateKeyBytes])

	sig := ffi.PrivateKeySign(*sk, msg)

	return sig[:], nil
}

func (blsSigner) Verify(sig []byte, a address.Address, msg []byte) error {
	payload := a.Payload()
	if sig == nil || len(sig) != ffi.SignatureBytes || len(payload) != ffi.PublicKeyBytes {
		return fmt.Errorf("bls signature failed to verify")
	}

	pk := new(PublicKey)
	copy(pk[:], payload[:ffi.PublicKeyBytes])

	sigS := new(Signature)
	copy(sigS[:], sig[:ffi.SignatureBytes])

	msgs := [1]ffi.Message{msg}
	pks := [1]PublicKey{*pk}

	if !ffi.HashVerify(sigS, msgs[:], pks[:]) {
		return fmt.Errorf("bls signature failed to verify")
	}

	return nil
}

func init() {
	crypto2.RegisterSignature(crypto.SigTypeBLS, blsSigner{})
}
