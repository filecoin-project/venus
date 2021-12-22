package testhelpers

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/pkg/crypto"
)

// MockSigner implements the Signer interface
type MockSigner struct {
	AddrKeyInfo map[address.Address]crypto.KeyInfo
	Addresses   []address.Address
	PubKeys     [][]byte
}

// NewMockSigner returns a new mock signer, capable of signing data with
// keys (addresses derived from) in keyinfo
func NewMockSigner(kis []crypto.KeyInfo) MockSigner {
	var ms MockSigner
	ms.AddrKeyInfo = make(map[address.Address]crypto.KeyInfo)
	for _, k := range kis {
		// extract public key
		pub, err := k.PublicKey()
		if err != nil {
			panic(err)
		}

		var newAddr address.Address
		if k.SigType == crypto.SigTypeSecp256k1 {
			newAddr, err = address.NewSecp256k1Address(pub)
		} else if k.SigType == crypto.SigTypeBLS {
			newAddr, err = address.NewBLSAddress(pub)
		}
		if err != nil {
			panic(err)
		}
		ms.Addresses = append(ms.Addresses, newAddr)
		ms.AddrKeyInfo[newAddr] = k
		ms.PubKeys = append(ms.PubKeys, pub)
	}
	return ms
}

// NewMockSignersAndKeyInfo is a convenience function to generate a mock
// signers with some keys.
func NewMockSignersAndKeyInfo(numSigners int) (MockSigner, []crypto.KeyInfo) {
	ki := MustGenerateKeyInfo(numSigners, 42)
	signer := NewMockSigner(ki)
	return signer, ki
}

// MustGenerateMixedKeyInfo produces m bls keys and n secp keys.
// BLS and Secp will be interleaved. The keys will be valid, but not deterministic.
func MustGenerateMixedKeyInfo(m int, n int) []crypto.KeyInfo {
	info := []crypto.KeyInfo{}
	for m > 0 && n > 0 {
		if m > 0 {
			ki, err := crypto.NewBLSKeyFromSeed(rand.Reader)
			if err != nil {
				panic(err)
			}
			info = append(info, ki)
			m--
		}

		if n > 0 {
			ki, err := crypto.NewSecpKeyFromSeed(rand.Reader)
			if err != nil {
				panic(err)
			}
			info = append(info, ki)
			n--
		}
	}
	return info
}

// MustGenerateBLSKeyInfo produces n distinct BLS keyinfos.
func MustGenerateBLSKeyInfo(n int, seed byte) []crypto.KeyInfo {
	token := bytes.Repeat([]byte{seed}, 512)
	var keyinfos []crypto.KeyInfo
	for i := 0; i < n; i++ {
		token[0] = byte(i)
		ki, err := crypto.NewBLSKeyFromSeed(bytes.NewReader(token))
		if err != nil {
			panic(err)
		}
		keyinfos = append(keyinfos, ki)
	}
	return keyinfos
}

// MustGenerateKeyInfo generates `n` distinct keyinfos using seed `seed`.
// The result is deterministic (for stable tests), don't use this for real keys!
func MustGenerateKeyInfo(n int, seed byte) []crypto.KeyInfo {
	token := bytes.Repeat([]byte{seed}, 512)
	var keyinfos []crypto.KeyInfo
	for i := 0; i < n; i++ {
		token[0] = byte(i)
		ki, err := crypto.NewSecpKeyFromSeed(bytes.NewReader(token))
		if err != nil {
			panic(err)
		}
		keyinfos = append(keyinfos, ki)
	}
	return keyinfos
}

// SignBytes cryptographically signs `data` using the  `addr`.
func (ms MockSigner) SignBytes(_ context.Context, data []byte, addr address.Address) (*crypto.Signature, error) {
	ki, ok := ms.AddrKeyInfo[addr]
	if !ok {
		return nil, errors.New("unknown address")
	}
	var sig *crypto.Signature
	err := ki.UsePrivateKey(func(privateKey []byte) error {
		var err error
		sig, err = crypto.Sign(data, privateKey, ki.SigType)

		return err
	})
	return sig, err
}

// HasAddress returns whether the signer can sign with this address
func (ms MockSigner) HasAddress(_ context.Context, addr address.Address) (bool, error) {
	return true, nil
}

// GetAddressForPubKey looks up a KeyInfo address associated with a given PublicKeyForSecpSecretKey for a MockSigner
func (ms MockSigner) GetAddressForPubKey(pk []byte) (address.Address, error) {
	var addr address.Address

	for _, ki := range ms.AddrKeyInfo {
		testPk, err := ki.PublicKey()
		if err != nil {
			return address.Undef, err
		}

		if bytes.Equal(testPk, pk) {
			addr, err := ki.Address()
			if err != nil {
				return addr, errors.New("could not fetch address")
			}
			return addr, nil
		}
	}
	return addr, errors.New("public key not found in wallet")
}
