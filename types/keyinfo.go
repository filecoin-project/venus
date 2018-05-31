package types

import (
	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"
)

func init() {
	cbor.RegisterCborType(KeyInfo{})
}

// KeyInfo is a key and its type used for signing
type KeyInfo struct {
	// Private key as bytes
	PrivateKey []byte
	// Curve used to generate private key
	Curve string
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
	if len(ki.PrivateKey) != len(other.PrivateKey) {
		return false
	}
	for i := range ki.PrivateKey {
		if ki.PrivateKey[i] != other.PrivateKey[i] {
			return false
		}
	}
	return true
}
