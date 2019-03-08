package types

import (
	"bytes"

	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/crypto"
)

func init() {
	cbor.RegisterCborType(KeyInfo{})
}

// KeyInfo is a key and its type used for signing.
type KeyInfo struct {
	// Private key.
	PrivateKey []byte `json:"privateKey"`
	// Curve used to generate private key.
	Curve string `json:"curve"`
}

// Unmarshal decodes raw cbor bytes into KeyInfo.
func (ki *KeyInfo) Unmarshal(b []byte) error {
	return cbor.DecodeInto(b, ki)
}

// Marshal KeyInfo into bytes.
func (ki *KeyInfo) Marshal() ([]byte, error) {
	return cbor.DumpObject(ki)
}

// Key returns the private key of KeyInfo
func (ki *KeyInfo) Key() []byte {
	return ki.PrivateKey
}

// Type returns the type of curve used to generate the private key
func (ki *KeyInfo) Type() string {
	return ki.Curve
}

// Equals returns true if the KeyInfo is equal to other.
func (ki *KeyInfo) Equals(other *KeyInfo) bool {
	if ki == nil && other == nil {
		return true
	}
	if ki == nil || other == nil {
		return false
	}
	if ki.Curve != other.Curve {
		return false
	}

	return bytes.Equal(ki.PrivateKey, other.PrivateKey)
}

// Address returns the address for this keyinfo
func (ki *KeyInfo) Address() (address.Address, error) {
	// TODO: Use the address type we are running on from the config.
	return address.NewSecp256k1Address(ki.PublicKey())
}

// PublicKey returns the public key part as uncompressed bytes.
func (ki *KeyInfo) PublicKey() []byte {
	return crypto.PublicKey(ki.PrivateKey)
}
