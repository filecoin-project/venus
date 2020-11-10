package crypto

import (
	"bytes"

	"github.com/filecoin-project/go-address"
	"github.com/pkg/errors"

	bls "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/venus/internal/pkg/encoding"
)

// KeyInfo is a key and its type used for signing.
type KeyInfo struct {
	// Private key.
	PrivateKey []byte `json:"privateKey"`
	// Cryptographic system used to generate private key.
	SigType SigType `json:"sigType"`
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
func (ki *KeyInfo) Type() SigType {
	return ki.SigType
}

// Equals returns true if the KeyInfo is equal to other.
func (ki *KeyInfo) Equals(other *KeyInfo) bool {
	if ki == nil && other == nil {
		return true
	}
	if ki == nil || other == nil {
		return false
	}
	if ki.SigType != other.SigType {
		return false
	}

	return bytes.Equal(ki.PrivateKey, other.PrivateKey)
}

// Address returns the address for this keyinfo
func (ki *KeyInfo) Address() (address.Address, error) {
	if ki.SigType == SigTypeBLS {
		return address.NewBLSAddress(ki.PublicKey())
	}
	if ki.SigType == SigTypeSecp256k1 {
		return address.NewSecp256k1Address(ki.PublicKey())
	}
	return address.Undef, errors.Errorf("can not generate address for unknown crypto system: %d", ki.SigType)
}

// Returns the public key part as uncompressed bytes.
func (ki *KeyInfo) PublicKey() []byte {
	if ki.SigType == SigTypeBLS {
		var blsPrivateKey bls.PrivateKey
		copy(blsPrivateKey[:], ki.PrivateKey)
		publicKey := bls.PrivateKeyPublicKey(blsPrivateKey)

		return publicKey[:]
	}
	if ki.SigType == SigTypeSecp256k1 {
		return PublicKeyForSecpSecretKey(ki.PrivateKey)
	}
	return []byte{}
}
