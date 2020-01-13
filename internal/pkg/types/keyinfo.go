package types

import (
	"bytes"

	"github.com/pkg/errors"

	bls "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

// KeyInfo is a key and its type used for signing.
type KeyInfo struct {
	// Private key.
	PrivateKey []byte `json:"privateKey"`
	// Cryptographic system used to generate private key.
	CryptSystem string `json:"cryptSystem"`
}

// Unmarshal decodes raw cbor bytes into KeyInfo.
func (ki *KeyInfo) Unmarshal(b []byte) error {
	return encoding.Decode(b, ki)
}

// Marshal KeyInfo into bytes.
func (ki *KeyInfo) Marshal() ([]byte, error) {
	return encoding.Encode(ki)
}

// Key returns the private key of KeyInfo
func (ki *KeyInfo) Key() []byte {
	return ki.PrivateKey
}

// Type returns the type of curve used to generate the private key
func (ki *KeyInfo) Type() string {
	return ki.CryptSystem
}

// Equals returns true if the KeyInfo is equal to other.
func (ki *KeyInfo) Equals(other *KeyInfo) bool {
	if ki == nil && other == nil {
		return true
	}
	if ki == nil || other == nil {
		return false
	}
	if ki.CryptSystem != other.CryptSystem {
		return false
	}

	return bytes.Equal(ki.PrivateKey, other.PrivateKey)
}

// Address returns the address for this keyinfo
func (ki *KeyInfo) Address() (address.Address, error) {
	if ki.CryptSystem == BLS {
		return address.NewBLSAddress(ki.PublicKey())
	}
	if ki.CryptSystem == SECP256K1 {
		return address.NewSecp256k1Address(ki.PublicKey())
	}
	return address.Undef, errors.Errorf("can not generate address for unknown crypto system: %s", ki.CryptSystem)
}

// PublicKey returns the public key part as uncompressed bytes.
func (ki *KeyInfo) PublicKey() []byte {
	if ki.CryptSystem == BLS {
		var blsPrivateKey bls.PrivateKey
		copy(blsPrivateKey[:], ki.PrivateKey)
		publicKey := bls.PrivateKeyPublicKey(blsPrivateKey)

		return publicKey[:]
	}
	if ki.CryptSystem == SECP256K1 {
		return crypto.PublicKey(ki.PrivateKey)
	}
	return []byte{}
}
