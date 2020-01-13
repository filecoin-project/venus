package postgenerator

import (
	ffi "github.com/filecoin-project/filecoin-ffi"
)

// PoStGenerator defines a method set used to generate PoSts
type PoStGenerator interface {
	GenerateEPostCandidates(sectorInfo ffi.SortedPublicSectorInfo, challengeSeed [ffi.CommitmentBytesLen]byte, faults []uint64) ([]ffi.Candidate, error)
	ComputeElectionPoSt(sectorInfo ffi.SortedPublicSectorInfo, challengeSeed []byte, winners []ffi.Candidate) ([]byte, error)
}
