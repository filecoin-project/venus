package types

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
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
type InvocResult struct {
	MsgCid         cid.Cid
	Msg            *UnsignedMessage
	MsgRct         *MessageReceipt
	GasCost        *MsgGasCost
	ExecutionTrace *ExecutionTrace
	Error          string
	Duration       time.Duration
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