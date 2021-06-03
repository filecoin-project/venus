package wallet

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/filecoin-project/go-state-types/crypto"
)

const StringEmpty = ""

//type MsgType string
type MsgType string

type MsgMeta struct {
	Type MsgType
	// Additional data related to what is signed. Should be verifiable with the
	// signed bytes (e.g. CID(Extra).Bytes() == toSign)
	Extra []byte
}

const (
	MTUnknown = MsgType("unknown")

	// Signing message CID. MsgMeta.Extra contains raw cbor message bytes
	MTChainMsg = MsgType("message")

	// Signing a blockheader. signing raw cbor block bytes (MsgMeta.Extra is empty)
	MTBlock = MsgType("block")

	// Signing a deal proposal. signing raw cbor proposal bytes (MsgMeta.Extra is empty)
	MTDealProposal = MsgType("dealproposal")
	// extra is nil, 'toSign' is cbor raw bytes of 'DrawRandomParams'
	//  following types follow above rule
	MTDrawRandomParam = MsgType("drawrandomparam")
	MTSignedVoucher   = MsgType("signedvoucher")
	MTStorageAsk      = MsgType("storageask")
	MTAskResponse     = MsgType("askresponse")
	MTNetWorkResponse = MsgType("networkresposne")

	// reference : storagemarket/impl/remotecli.go:330
	// sign storagemarket.ClientDeal.ProposalCid,
	// MsgMeta.Extra is nil, 'toSign' is market.ClientDealProposal
	// storagemarket.ClientDeal.ProposalCid equals cborutil.AsIpld(market.ClientDealProposal).Cid()
	MTClientDeal = MsgType("clientdeal")

	MTProviderDealState = MsgType("providerdealstate")
)

// KeyType defines a type of a key
type KeyType string

const (
	KTUnknown   KeyType = "unknown"
	KTBLS       KeyType = "bls"
	KTSecp256k1 KeyType = "secp256k1"
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

type KeyInfo struct {
	Type       KeyType
	PrivateKey []byte
}

type SigType = crypto.SigType

const (
	SigTypeUnknown = SigType(math.MaxUint8)

	SigTypeSecp256k1 = SigType(iota)
	SigTypeBLS
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
