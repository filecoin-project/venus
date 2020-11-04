package block

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/specactors/builtin"
	"github.com/filecoin-project/go-state-types/abi"
)

type MiningBaseInfo struct {
	MinerPower      abi.StoragePower
	NetworkPower    abi.StoragePower
	Sectors         []builtin.SectorInfo
	WorkerKey       address.Address
	SectorSize      abi.SectorSize
	PrevBeaconEntry BeaconEntry
	BeaconEntries   []BeaconEntry
	HasMinPower     bool
}
