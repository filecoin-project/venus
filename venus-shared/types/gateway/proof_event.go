package gateway

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"

	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
)

type MinerState struct {
	Connections     []*ConnectState
	ConnectionCount int
}

type ProofRegisterPolicy struct {
	MinerAddress address.Address
}

type ComputeProofRequest struct {
	SectorInfos []builtin.ExtendedSectorInfo
	Rand        abi.PoStRandomness
	Height      abi.ChainEpoch
	NWVersion   network.Version
}
