package syncer

import (
	"time"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"
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

type SyncState struct {
	ActiveSyncs []ActiveSync

	VMApplied uint64
}

type SyncStateStage int

type ActiveSync struct {
	WorkerID uint64
	Base     *block.TipSet
	Target   *block.TipSet

	Stage  SyncStateStage
	Height abi.ChainEpoch

	Start   time.Time
	End     time.Time
	Message string
}
