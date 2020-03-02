package postgenerator

import (
	ffi "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/specs-actors/actors/abi"
)

// SectorBuilderBackEnd uses the go-sectorbuilder package to
// generate PoSts.
type SectorBuilderBackEnd struct {
	builder SectorBuilderAPI
}

// SectorBuilderAPI defines a subset of the sectorbuilder.Interface used by
// the SectorBuilderBackEnd.
type SectorBuilderAPI interface {
	GenerateEPostCandidates(sectorInfo []abi.SectorInfo, challengeSeed abi.PoStRandomness, faults []abi.SectorNumber) ([]ffi.PoStCandidateWithTicket, error)
	GenerateFallbackPoSt(sectorInfo []abi.SectorInfo, challengeSeed abi.PoStRandomness, faults []abi.SectorNumber) ([]ffi.PoStCandidateWithTicket, []abi.PoStProof, error)
	ComputeElectionPoSt(sectorInfo []abi.SectorInfo, challengeSeed abi.PoStRandomness, winners []abi.PoStCandidate) ([]abi.PoStProof, error)
}

// NewSectorBuilderBackEnd produces a SectorBuilderBackEnd, which uses the
// go-sectorbuilder package to generate PoSts.
func NewSectorBuilderBackEnd(s SectorBuilderAPI) *SectorBuilderBackEnd {
	return &SectorBuilderBackEnd{builder: s}
}

// GenerateEPostCandidates produces election PoSt candidates from the provided
// proving set.
func (s *SectorBuilderBackEnd) GenerateEPostCandidates(sectorInfo []abi.SectorInfo, challengeSeed abi.PoStRandomness, faults []abi.SectorNumber) ([]ffi.PoStCandidateWithTicket, error) {
	return s.builder.GenerateEPostCandidates(sectorInfo, challengeSeed, faults)
}

// GenerateFallbackPoSt generates a fallback PoSt and returns the proof and
// all candidates used when generating it.
func (s *SectorBuilderBackEnd) GenerateFallbackPoSt(sectorInfo []abi.SectorInfo, challengeSeed abi.PoStRandomness, faults []abi.SectorNumber) ([]ffi.PoStCandidateWithTicket, []abi.PoStProof, error) {
	return s.builder.GenerateFallbackPoSt(sectorInfo, challengeSeed, faults)
}

// ComputeElectionPoSt produces a new (election) PoSt proof.
func (s *SectorBuilderBackEnd) ComputeElectionPoSt(sectorInfo []abi.SectorInfo, challengeSeed abi.PoStRandomness, winners []abi.PoStCandidate) ([]abi.PoStProof, error) {
	return s.builder.ComputeElectionPoSt(sectorInfo, challengeSeed, winners)
}
