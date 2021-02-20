package remotewallet

import (
	"encoding/json"
	"fmt"
	"github.com/filecoin-project/go-state-types/crypto"
)

type MsgType string
type MsgMeta struct {
	Type MsgType
	// Additional data related to what is signed. Should be verifiable with the
	// signed bytes (e.g. CID(Extra).Bytes() == toSign)
	Extra []byte
}
type Signature = crypto.Signature

// KeyType defines a type of a key
type KeyType string

type KeyInfo struct {
	Type       KeyType
	PrivateKey []byte
}

const (
	KTUnknown         KeyType = "unknown"
	KTBLS             KeyType = "bls"
	KTSecp256k1       KeyType = "secp256k1"
	KTSecp256k1Ledger KeyType = "secp256k1-ledger"
)

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
