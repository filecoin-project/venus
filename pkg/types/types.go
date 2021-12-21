package types

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/constants"

	"github.com/filecoin-project/venus/venus-shared/actors/builtin/market"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/miner"
)

type EstimateMessage struct {
	Msg  *Message
	Spec *MessageSendSpec
}

type EstimateResult struct {
	Msg *Message
	Err string
}

type MessageSendSpec struct {
	MaxFee            abi.TokenAmount
	GasOverEstimation float64
}

var DefaultMessageSendSpec = MessageSendSpec{
	// MaxFee of 0.1FIL
	MaxFee: abi.NewTokenAmount(int64(constants.FilecoinPrecision) / 10),
}

func (ms *MessageSendSpec) Get() MessageSendSpec {
	if ms == nil {
		return DefaultMessageSendSpec
	}

	return *ms
}

type ChainSectorInfo struct {
	Info miner.SectorOnChainInfo
	ID   abi.SectorNumber
}

type DealCollateralBounds struct {
	Min abi.TokenAmount
	Max abi.TokenAmount
}

type MarketDeal struct {
	Proposal market.DealProposal
	State    market.DealState
}

type CirculatingSupply struct {
	FilVested      abi.TokenAmount
	FilMined       abi.TokenAmount
	FilBurnt       abi.TokenAmount
	FilLocked      abi.TokenAmount
	FilCirculating abi.TokenAmount
}

type NetworkPower struct {
	RawBytePower         abi.StoragePower
	QualityAdjustedPower abi.StoragePower
	MinerCount           int64
	MinPowerMinerCount   int64
}
