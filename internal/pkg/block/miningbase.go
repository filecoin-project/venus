package block

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/internal/pkg/specactors/builtin"
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
