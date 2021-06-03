// Code from github.com/filecoin-project/venus-wallet/core/types.go. DO NOT EDIT.

package wallet

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
)

const StringEmpty = ""

type SigType = crypto.SigType

const (
	SigTypeUnknown = SigType(math.MaxUint8)

	SigTypeSecp256k1 = SigType(iota)
	SigTypeBLS
)

type AddressScope struct {
	Root      bool      // is root auth,  true : can get all addresses in the wallet
	Addresses []Address // when root==false, should fill a scope of wallet addresses
}

type Address = address.Address
type Signature = crypto.Signature
type TokenAmount = abi.TokenAmount
type MethodNum = abi.MethodNum
type Cid = cid.Cid

type KeyInfo struct {
	Type       KeyType
	PrivateKey []byte
}

var (
	NilAddress = Address{}
)

type MsgType string
type MsgMeta struct {
	Type MsgType
	// Additional data related to what is signed. Should be verifiable with the
	// signed bytes (e.g. CID(Extra).Bytes() == toSign)
	Extra []byte
}

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

const (
	KTUnknown   KeyType = "unknown"
	KTBLS       KeyType = "bls"
	KTSecp256k1 KeyType = "secp256k1"
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
