package crypto

import (
	"io"

	"github.com/filecoin-project/go-state-types/crypto"
)

//
// Abstract SECP and BLS crypto operations.
//

// NewSecpKeyFromSeed generates a new key from the given reader.
func NewSecpKeyFromSeed(seed io.Reader) (KeyInfo, error) {
	k, err := sigs[crypto.SigTypeSecp256k1].GenPrivateFromSeed(seed)
	if err != nil {
		return KeyInfo{}, err
	}
	ki := &KeyInfo{
		SigType: SigTypeSecp256k1,
	}
	ki.SetPrivateKey(k)
	copy(k, make([]byte, len(k))) //wipe with zero bytes
	return *ki, nil
}

func NewBLSKeyFromSeed(seed io.Reader) (KeyInfo, error) {
	k, err := sigs[crypto.SigTypeBLS].GenPrivateFromSeed(seed)
	if err != nil {
		return KeyInfo{}, err
	}
	ki := &KeyInfo{
		SigType: SigTypeBLS,
	}
	ki.SetPrivateKey(k)
	copy(k, make([]byte, len(k))) //wipe with zero bytes
	return *ki, nil
}
