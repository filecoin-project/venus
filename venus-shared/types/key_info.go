package types

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/filecoin-project/go-state-types/crypto"
)

var (
	ErrKeyInfoNotFound = fmt.Errorf("key info not found")
	ErrKeyExists       = fmt.Errorf("key already exists")
)

// KeyType defines a type of a key
type KeyType string

func (kt *KeyType) UnmarshalJSON(bb []byte) error {
	{
		// first option, try unmarshaling as string
		var s string
		err := json.Unmarshal(bb, &s)
		if err == nil {
			*kt = KeyType(s)
			return nil
		}
	}

	{
		var b byte
		err := json.Unmarshal(bb, &b)
		if err != nil {
			return fmt.Errorf("could not unmarshal KeyType either as string nor integer: %w", err)
		}
		bst := crypto.SigType(b)

		switch bst {
		case crypto.SigTypeBLS:
			*kt = KTBLS
		case crypto.SigTypeSecp256k1:
			*kt = KTSecp256k1
		default:
			return fmt.Errorf("unknown sigtype: %d", bst)
		}
		return nil
	}
}

type SigType = crypto.SigType

const (
	SigTypeUnknown = SigType(math.MaxUint8)

	SigTypeSecp256k1 = SigType(iota)
	SigTypeBLS
)

const (
	KTUnknown         KeyType = "unknown"
	KTBLS             KeyType = "bls"
	KTSecp256k1       KeyType = "secp256k1"
	KTSecp256k1Ledger KeyType = "secp256k1-ledger"
)

func KeyType2Sign(kt KeyType) SigType {
	switch kt {
	case KTSecp256k1:
		return SigTypeSecp256k1
	case KTBLS:
		return SigTypeBLS
	default:
		return SigTypeUnknown
	}
}

func SignType2Key(kt SigType) KeyType {
	switch kt {
	case SigTypeSecp256k1:
		return KTSecp256k1
	case SigTypeBLS:
		return KTBLS
	default:
		return KTUnknown
	}
}

// KeyInfo is used for storing keys in KeyStore
type KeyInfo struct {
	Type       KeyType
	PrivateKey []byte
}
