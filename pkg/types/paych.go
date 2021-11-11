package types

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/types/internal"
	"github.com/ipfs/go-cid"
	"time"
)

type PaychStatus struct {
	ControlAddr address.Address
	Direction   PCHDir
}
type PCHDir int

const (
	PCHUndef PCHDir = iota
	PCHInbound
	PCHOutbound
)

type TipsetInvokeResult struct {
	Key       TipSetKey
	Epoch     abi.ChainEpoch
	StateRoot cid.Cid
	MsgRets   []*InvocResult
}

type InvocResult struct {
	MsgCid cid.Cid // if returned by implicitly called message MsgCid is undefined.

	// this field can be an *vmcontext.VMMessage(implicitly called message) or *UnsignedMessage,
	Msg interface{} `json:",omitempty"`

	StateRootAfterApply cid.Cid // the state root after message apply

	MsgRct         *internal.MessageReceipt `json:",omitempty"`
	GasCost        *MsgGasCost
	ExecutionTrace *ExecutionTrace // if returned by implicitly called message member field 'Msg' is nil
	Error          string          `json:",omitempty"`
	Duration       time.Duration   `json:",omitempty"`
}

type MsgGasCost struct {
	Message            cid.Cid // Can be different than requested, in case it was replaced, but only gas values changed
	GasUsed            abi.TokenAmount
	BaseFeeBurn        abi.TokenAmount
	OverEstimationBurn abi.TokenAmount
	MinerPenalty       abi.TokenAmount
	MinerTip           abi.TokenAmount
	Refund             abi.TokenAmount
	TotalCost          abi.TokenAmount
}
