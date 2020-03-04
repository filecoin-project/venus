package postgenerator

import (
	ffi "github.com/filecoin-project/filecoin-ffi"

	"github.com/filecoin-project/specs-actors/actors/abi"
)

// PoStGenerator defines a method set used to generate PoSts
type PoStGenerator interface {
	GenerateEPostCandidates(sectorInfo []abi.SectorInfo, challengeSeed abi.PoStRandomness, faults []abi.SectorNumber) ([]ffi.PoStCandidateWithTicket, error)
	ComputeElectionPoSt(sectorInfo []abi.SectorInfo, challengeSeed abi.PoStRandomness, winners []abi.PoStCandidate) ([]abi.PoStProof, error)
}
