package types

import (
	"bytes"
	"crypto/ecdsa"
	"fmt"

	cbor "gx/ipfs/QmV6BQ6fFCf9eFHDuRxvguvqfKLZtZrxthgZvDfRCs4tMN/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/crypto"
	cu "github.com/filecoin-project/go-filecoin/crypto/util"
)

func init() {
	cbor.RegisterCborType(KeyInfo{})
}

// KeyInfo is a key and its type used for signing
type KeyInfo struct {
	// Private key as bytes
	PrivateKey []byte `json:"privateKey"`
	// Curve used to generate private key
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
func (ki *KeyInfo) Address() (Address, error) {
	pub, err := ki.PublicKey()
	if err != nil {
		return Address{}, err
	}

	addrHash := AddressHash(pub)

	// TODO: Use the address type we are running on from the config.
	return NewMainnetAddress(addrHash), nil
}

// PublicKey returns the public key part as uncompressed bytes.
func (ki *KeyInfo) PublicKey() ([]byte, error) {
	prv, err := crypto.BytesToECDSA(ki.Key())
	if err != nil {
		return nil, err
	}

	pub, ok := prv.Public().(*ecdsa.PublicKey)
	if !ok {
		// means a something is wrong with key generation
		return nil, fmt.Errorf("unknown public key type")
	}

	return cu.SerializeUncompressed(pub), nil
}
