package postgenerator

import (
	"github.com/filecoin-project/go-sectorbuilder"
	"github.com/filecoin-project/specs-actors/actors/abi"
)

// PoStGenerator defines a method set used to generate PoSts
type PoStGenerator interface {
	GenerateEPostCandidates(sectorInfo sectorbuilder.SortedPublicSectorInfo, challengeSeed [sectorbuilder.CommLen]byte, faults []abi.SectorNumber) ([]sectorbuilder.EPostCandidate, error)
	ComputeElectionPoSt(sectorInfo sectorbuilder.SortedPublicSectorInfo, challengeSeed []byte, winners []sectorbuilder.EPostCandidate) ([]byte, error)
}
