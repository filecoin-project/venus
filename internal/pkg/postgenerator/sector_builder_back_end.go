package postgenerator

import (
	"github.com/filecoin-project/go-sectorbuilder"
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
	GenerateEPostCandidates(sectorInfo sectorbuilder.SortedPublicSectorInfo, challengeSeed [sectorbuilder.CommLen]byte, faults []abi.SectorNumber) ([]sectorbuilder.EPostCandidate, error)
	GenerateFallbackPoSt(sectorbuilder.SortedPublicSectorInfo, [sectorbuilder.CommLen]byte, []abi.SectorNumber) ([]sectorbuilder.EPostCandidate, []byte, error)
	ComputeElectionPoSt(sectorInfo sectorbuilder.SortedPublicSectorInfo, challengeSeed []byte, winners []sectorbuilder.EPostCandidate) ([]byte, error)
}

// NewSectorBuilderBackEnd produces a SectorBuilderBackEnd, which uses the
// go-sectorbuilder package to generate PoSts.
func NewSectorBuilderBackEnd(s SectorBuilderAPI) *SectorBuilderBackEnd {
	return &SectorBuilderBackEnd{builder: s}
}

// GenerateEPostCandidates produces election PoSt candidates from the provided
// proving set.
func (s *SectorBuilderBackEnd) GenerateEPostCandidates(sectorInfo sectorbuilder.SortedPublicSectorInfo, challengeSeed [sectorbuilder.CommLen]byte, faults []abi.SectorNumber) ([]sectorbuilder.EPostCandidate, error) {
	return s.builder.GenerateEPostCandidates(sectorInfo, challengeSeed, faults)
}

// GenerateFallbackPoSt generates a fallback PoSt and returns the proof and
// all candidates used when generating it.
func (s *SectorBuilderBackEnd) GenerateFallbackPoSt(sectorInfo sectorbuilder.SortedPublicSectorInfo, challengeSeed [sectorbuilder.CommLen]byte, faults []abi.SectorNumber) ([]sectorbuilder.EPostCandidate, []byte, error) {
	return s.builder.GenerateFallbackPoSt(sectorInfo, challengeSeed, faults)
}

// ComputeElectionPoSt produces a new (election) PoSt proof.
func (s *SectorBuilderBackEnd) ComputeElectionPoSt(sectorInfo sectorbuilder.SortedPublicSectorInfo, challengeSeed []byte, winners []sectorbuilder.EPostCandidate) ([]byte, error) {
	return s.builder.ComputeElectionPoSt(sectorInfo, challengeSeed, winners)
}
