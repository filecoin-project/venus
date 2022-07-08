package messager

import (
	"encoding/json"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/ipfs/go-cid"

	shared "github.com/filecoin-project/venus/venus-shared/types"
)

//						---> FailedMsg <------
//					    |					 |
// 				UnFillMsg ---------------> FillMsg --------> OnChainMsg
//						|					 |
//		 NoWalletMsg <---				     ---->ReplacedMsg
//

type MessageState int

const (
	UnKnown MessageState = iota
	UnFillMsg
	FillMsg
	OnChainMsg
	FailedMsg
	ReplacedMsg
	NoWalletMsg
)

func (mst MessageState) String() string {
	switch mst {
	case UnFillMsg:
		return "UnFillMsg"
	case FillMsg:
		return "FillMsg"
	case OnChainMsg:
		return "OnChainMsg"
	case FailedMsg:
		return "Failed"
	case ReplacedMsg:
		return "ReplacedMsg"
	case NoWalletMsg:
		return "NoWalletMsg"
	default:
		return "UnKnown"
	}
}

func MessageStateToString(state MessageState) string {
	return state.String()
}

type MessageWithUID struct {
	UnsignedMessage shared.Message
	ID              string
}

func FromUnsignedMessage(unsignedMsg shared.Message) *Message {
	return &Message{
		Message: unsignedMsg,
	}
}

type Message struct {
	ID string

	UnsignedCid *cid.Cid
	SignedCid   *cid.Cid
	shared.Message
	Signature *crypto.Signature

	Height     int64
	Confidence int64
	Receipt    *shared.MessageReceipt
	TipSetKey  shared.TipSetKey
	Meta       *SendSpec
	WalletName string
	FromUser   string

	State MessageState

	CreatedAt time.Time
	UpdatedAt time.Time
}

//todo ignore use message MarshalJSON method
func (m *Message) MarshalJSON() ([]byte, error) {
	type msg struct {
		Version    uint64
		To         address.Address
		From       address.Address
		Nonce      uint64
		Value      abi.TokenAmount
		GasLimit   int64
		GasFeeCap  abi.TokenAmount
		GasPremium abi.TokenAmount
		Method     abi.MethodNum
		Params     []byte
	}
	type fMsg struct {
		ID string

		UnsignedCid *cid.Cid
		SignedCid   *cid.Cid
		msg
		Signature *crypto.Signature

		Height     int64
		Confidence int64
		Receipt    *shared.MessageReceipt
		TipSetKey  shared.TipSetKey
		Meta       *SendSpec
		WalletName string
		FromUser   string

		State MessageState

		CreatedAt time.Time
		UpdatedAt time.Time
	}
	return json.Marshal(fMsg{
		ID:          m.ID,
		UnsignedCid: m.UnsignedCid,
		SignedCid:   m.SignedCid,
		msg: msg{
			Version:    m.Message.Version,
			To:         m.Message.To,
			From:       m.Message.From,
			Nonce:      m.Message.Nonce,
			Value:      m.Message.Value,
			GasLimit:   m.Message.GasLimit,
			GasFeeCap:  m.Message.GasFeeCap,
			GasPremium: m.Message.GasPremium,
			Method:     m.Message.Method,
			Params:     m.Message.Params,
		},
		Signature:  m.Signature,
		Height:     m.Height,
		Confidence: m.Confidence,
		Receipt:    m.Receipt,
		TipSetKey:  m.TipSetKey,
		Meta:       m.Meta,
		WalletName: m.WalletName,
		FromUser:   m.FromUser,
		State:      m.State,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	})
}

type ReplacMessageParams struct {
	ID             string
	Auto           bool
	MaxFee         abi.TokenAmount
	GasLimit       int64
	GasPremium     abi.TokenAmount
	GasFeecap      abi.TokenAmount
	GasOverPremium float64
}
