package types

import (
	"bytes"
	"io"
	"math/rand"

	"github.com/filecoin-project/go-filecoin/crypto"
)

const (
	// SECP256K1 is a curve used to compute private keys
	SECP256K1 = "secp256k1"
)

// MustGenerateKeyInfo generates a slice of KeyInfo size `n` with seed `seed`
func MustGenerateKeyInfo(n int, seed io.Reader) []KeyInfo {
	var keyinfos []KeyInfo
	for i := 0; i < n; i++ {
		prv, err := crypto.GenerateKeyFromSeed(seed)
		if err != nil {
			panic(err)
		}

		ki := &KeyInfo{
			PrivateKey: prv,
			Curve:      SECP256K1,
		}
		keyinfos = append(keyinfos, *ki)
	}
	return keyinfos
}

// GenerateKeyInfoSeed returns a random to be passed to MustGenerateKeyInfo
func GenerateKeyInfoSeed() io.Reader {
	token := make([]byte, 512)
	rand.Read(token)
	return bytes.NewReader(token)
}
