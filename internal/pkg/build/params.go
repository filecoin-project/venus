package build

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/specs-actors/actors/runtime/proof"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
)

const NewestNetworkVersion = network.Version4


const FilecoinPrecision = uint64(1_000_000_000_000_000_000)

type MiningBaseInfo struct {
	MinerPower      abi.StoragePower
	NetworkPower    abi.StoragePower
	Sectors         []proof.SectorInfo
	WorkerKey       address.Address
	SectorSize      abi.SectorSize
	PrevBeaconEntry block.BeaconEntry
	BeaconEntries   []block.BeaconEntry
	HasMinPower     bool
}

