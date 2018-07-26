package types

import (
	"github.com/filecoin-project/go-filecoin/crypto"
)

const (
	// SECP256K1 is a curve used to compute private keys
	SECP256K1 = "secp256k1"
)

// MustGenerateKeyInfo generates a slice of KeyInfo size `n`
func MustGenerateKeyInfo(n int) []KeyInfo {
	var keyinfos []KeyInfo
	for i := 0; i < n; i++ {
		prv, err := crypto.GenerateKey()
		if err != nil {
			panic(err)
		}

		ki := &KeyInfo{
			PrivateKey: crypto.ECDSAToBytes(prv),
			Curve:      SECP256K1,
		}
		keyinfos = append(keyinfos, *ki)
	}
	return keyinfos
}
