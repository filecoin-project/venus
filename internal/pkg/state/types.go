package state

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/specactors/builtin/market"
	"github.com/filecoin-project/go-filecoin/internal/pkg/specactors/builtin/miner"
	"github.com/filecoin-project/go-state-types/abi"
)

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
