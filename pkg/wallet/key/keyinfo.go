package key

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/awnumar/memguard"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/crypto"
	acrypto "github.com/filecoin-project/venus/pkg/crypto"
	_ "github.com/filecoin-project/venus/pkg/crypto/bls"
	_ "github.com/filecoin-project/venus/pkg/crypto/delegated"
	_ "github.com/filecoin-project/venus/pkg/crypto/secp"
	"github.com/filecoin-project/venus/venus-shared/types"
	logging "github.com/ipfs/go-log/v2"
	"github.com/pkg/errors"
)

var log = logging.Logger("keyinfo")

// KeyInfo is a key and its type used for signing.
type KeyInfo struct { // nolint
	// Private key.
	PrivateKey *memguard.Enclave `json:"privateKey"`
	// Cryptographic system used to generate private key.
	SigType types.SigType `json:"type"`
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
		// compatible with lotus
		st := k.SigType.(string)
		if st == string(types.KTBLS) {
			ki.SigType = crypto.SigTypeBLS
		} else if st == string(types.KTSecp256k1) {
			ki.SigType = crypto.SigTypeSecp256k1
		} else if st == string(types.KTDelegated) {
			ki.SigType = crypto.SigTypeDelegated
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
	ki.SetPrivateKey(k.PrivateKey)

	return nil
}

func (ki KeyInfo) MarshalJSON() ([]byte, error) {
	var err error
	var b []byte
	err = ki.UsePrivateKey(func(privateKey []byte) error {
		k := keyInfo{}
		k.PrivateKey = privateKey
		if ki.SigType == crypto.SigTypeBLS {
			k.SigType = types.KTBLS
		} else if ki.SigType == crypto.SigTypeSecp256k1 {
			k.SigType = types.KTSecp256k1
		} else if ki.SigType == crypto.SigTypeDelegated {
			k.SigType = types.KTDelegated
		} else {
			return fmt.Errorf("unsupport keystore types %T", k.SigType)
		}
		b, err = json.Marshal(k)
		return err
	})

	return b, err
}

// Key returns the private key of KeyInfo
// This method makes the key escape from memguard's protection, so use caution
func (ki *KeyInfo) Key() []byte {
	var pk []byte
	err := ki.UsePrivateKey(func(privateKey []byte) error {
		pk = make([]byte, len(privateKey))
		copy(pk, privateKey[:])
		return nil
	})
	if err != nil {
		log.Errorf("got private key failed %v", err)
		return []byte{}
	}
	return pk
}

// Type returns the type of curve used to generate the private key
func (ki *KeyInfo) Type() types.SigType {
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

	pk, err := ki.PrivateKey.Open()
	if err != nil {
		return false
	}
	defer pk.Destroy()

	otherPK, err := other.PrivateKey.Open()
	if err != nil {
		return false
	}
	defer otherPK.Destroy()

	return bytes.Equal(pk.Bytes(), otherPK.Bytes())
}

// Address returns the address for this keyinfo
func (ki *KeyInfo) Address() (address.Address, error) {
	pubKey, err := ki.PublicKey()
	if err != nil {
		return address.Undef, err
	}
	if ki.SigType == types.SigTypeBLS {
		return address.NewBLSAddress(pubKey)
	}
	if ki.SigType == types.SigTypeSecp256k1 {
		return address.NewSecp256k1Address(pubKey)
	}
	if ki.SigType == types.SigTypeDelegated {
		// Transitory Delegated signature verification as per FIP-0055
		ethAddr, err := types.EthAddressFromPubKey(pubKey)
		if err != nil {
			return address.Undef, fmt.Errorf("failed to calculate Eth address from public key: %w", err)
		}
		ea, err := types.CastEthAddress(ethAddr)
		if err != nil {
			return address.Undef, fmt.Errorf("failed to create ethereum address from bytes: %w", err)
		}

		// return address.NewDelegatedAddress(builtin.EthereumAddressManagerActorID, hasher.Sum(nil)[12:])
		return ea.ToFilecoinAddress()
	}

	return address.Undef, errors.Errorf("can not generate address for unknown crypto system: %d", ki.SigType)
}

// Returns the public key part as uncompressed bytes.
func (ki *KeyInfo) PublicKey() ([]byte, error) {
	var pubKey []byte
	err := ki.UsePrivateKey(func(privateKey []byte) error {
		var err error
		pubKey, err = acrypto.ToPublic(ki.SigType, privateKey)
		return err
	})

	return pubKey, err
}

func (ki *KeyInfo) UsePrivateKey(f func([]byte) error) error {
	buf, err := ki.PrivateKey.Open()
	if err != nil {
		return err
	}
	defer buf.Destroy()

	return f(buf.Bytes())
}

func (ki *KeyInfo) SetPrivateKey(privateKey []byte) {
	// will wipes privateKey with zeroes
	ki.PrivateKey = memguard.NewEnclave(privateKey)
}

// NewSecpKeyFromSeed generates a new key from the given reader.
func NewSecpKeyFromSeed(seed io.Reader) (KeyInfo, error) {
	k, err := acrypto.GenerateFromSeed(crypto.SigTypeSecp256k1, seed)
	if err != nil {
		return KeyInfo{}, err
	}
	ki := &KeyInfo{
		SigType: types.SigTypeSecp256k1,
	}
	ki.SetPrivateKey(k)
	copy(k, make([]byte, len(k))) // wipe with zero bytes
	return *ki, nil
}

func NewBLSKeyFromSeed(seed io.Reader) (KeyInfo, error) {
	k, err := acrypto.GenerateFromSeed(crypto.SigTypeBLS, seed)
	if err != nil {
		return KeyInfo{}, err
	}
	ki := &KeyInfo{
		SigType: types.SigTypeBLS,
	}
	ki.SetPrivateKey(k)
	copy(k, make([]byte, len(k))) // wipe with zero bytes
	return *ki, nil
}

func NewDelegatedKeyFromSeed(seed io.Reader) (KeyInfo, error) {
	k, err := acrypto.GenerateFromSeed(crypto.SigTypeDelegated, seed)
	if err != nil {
		return KeyInfo{}, err
	}
	ki := &KeyInfo{
		SigType: types.SigTypeDelegated,
	}
	ki.SetPrivateKey(k)
	copy(k, make([]byte, len(k))) // wipe with zero bytes
	return *ki, nil
}
