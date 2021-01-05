package crypto

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/filecoin-project/go-state-types/crypto"

	"github.com/filecoin-project/go-address"
	"github.com/pkg/errors"

	bls "github.com/filecoin-project/filecoin-ffi"
)

const (
	stBLS       = "bls"
	stSecp256k1 = "secp256k1"
)

// KeyInfo is a key and its type used for signing.
type KeyInfo struct {
	// Private key.
	PrivateKey []byte `json:"privateKey"`
	// Cryptographic system used to generate private key.
	SigType SigType `json:"type"`
}

type keyInfo struct {
	// Private key.
	PrivateKey []byte `json:"privateKey"`
	// Cryptographic system used to generate private key.
	SigType interface{} `json:"type"`
}

func (ki *KeyInfo) UnmarshalJSON(data []byte) error {
	k := keyInfo{}
	err := json.Unmarshal(data, &k)
	if err != nil {
		return err
	}

	switch k.SigType.(type) {
	case string:
		st := k.SigType.(string)
		if st == stBLS {
			ki.SigType = crypto.SigTypeBLS
		} else if st == stSecp256k1 {
			ki.SigType = crypto.SigTypeSecp256k1
		} else {
			return fmt.Errorf("unknown sig type value: %s", st)
		}
	case byte:
		ki.SigType = crypto.SigType(k.SigType.(byte))
	case float64:
		ki.SigType = crypto.SigType(k.SigType.(float64))
	case int:
		ki.SigType = crypto.SigType(k.SigType.(int))
	case int64:
		ki.SigType = crypto.SigType(k.SigType.(int64))
	default:
		return fmt.Errorf("unknown sig type: %T", k.SigType)
	}
	ki.PrivateKey = k.PrivateKey

	return nil
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
