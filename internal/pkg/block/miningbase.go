package block

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/actors/runtime/proof"
)

type MiningBaseInfo struct {
	MinerPower      abi.StoragePower
	NetworkPower    abi.StoragePower
	Sectors         []proof.SectorInfo
	WorkerKey       address.Address
	SectorSize      abi.SectorSize
	PrevBeaconEntry BeaconEntry
	BeaconEntries   []BeaconEntry
	HasMinPower     bool
}
