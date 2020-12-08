package syncer

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"
	"time"
)

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

type InvocResult struct {
	MsgCid         cid.Cid
	Msg            *types.UnsignedMessage
	MsgRct         *types.MessageReceipt
	GasCost        MsgGasCost
	ExecutionTrace types.ExecutionTrace
	Error          string
	Duration       time.Duration
}
