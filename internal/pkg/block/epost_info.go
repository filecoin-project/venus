package block

import (
	ffi "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

func init() {
	encoding.RegisterIpldCborType(EPoStCandidate{})
	encoding.RegisterIpldCborType(EPoStInfo{})
}

// EPoStInfo wraps all data needed to verify an election post proof
type EPoStInfo struct {
	PoStProof      types.PoStProof
	PoStRandomness VRFPi
	Winners        []EPoStCandidate
}

// EPoStCandidate wraps the input data needed to verify an election PoSt
type EPoStCandidate struct {
	PartialTicket        []byte
	SectorID             types.Uint64
	SectorChallengeIndex types.Uint64
}

// NewEPoStCandidate constructs an epost candidate from data
func NewEPoStCandidate(sID uint64, pt []byte, sci uint64) EPoStCandidate {
	return EPoStCandidate{
		SectorID:             types.Uint64(sID),
		PartialTicket:        pt,
		SectorChallengeIndex: types.Uint64(sci),
	}
}

// NewEPoStInfo constructs an epost info from data
func NewEPoStInfo(proof types.PoStProof, rand VRFPi, winners ...EPoStCandidate) EPoStInfo {
	return EPoStInfo{
		Winners:        winners,
		PoStProof:      proof,
		PoStRandomness: rand,
	}
}

// ToFFICandidate converts a block.EPoStCandidate to a ffi candidate
func (x *EPoStCandidate) ToFFICandidate() ffi.Candidate {
	var pt [32]byte
	copy(pt[:], x.PartialTicket)

	return ffi.Candidate{
		SectorID:             uint64(x.SectorID),
		PartialTicket:        pt,
		Ticket:               [32]byte{}, // note: this field is ignored
		SectorChallengeIndex: uint64(x.SectorChallengeIndex),
	}
}

// ToFFICandidates converts several block.EPoStCandidate to several ffi candidate
func ToFFICandidates(candidates ...EPoStCandidate) []ffi.Candidate {
	out := make([]ffi.Candidate, len(candidates))
	for idx, c := range candidates {
		out[idx] = c.ToFFICandidate()
	}

	return out
}

// FromFFICandidate converts a Candidate to an EPoStCandidate
func FromFFICandidate(candidate ffi.Candidate) EPoStCandidate {
	return EPoStCandidate{
		PartialTicket:        candidate.PartialTicket[:],
		SectorID:             types.Uint64(candidate.SectorID),
		SectorChallengeIndex: types.Uint64(candidate.SectorChallengeIndex),
	}
}

// FromFFICandidates converts a variable number of Candidate to a slice of
// EPoStCandidate
func FromFFICandidates(candidates ...ffi.Candidate) []EPoStCandidate {
	out := make([]EPoStCandidate, len(candidates))
	for idx, c := range candidates {
		out[idx] = FromFFICandidate(c)
	}

	return out
}
