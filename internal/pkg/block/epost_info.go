package block

import (
	ffi "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/specs-actors/actors/abi"
)

func init() {
	encoding.RegisterIpldCborType(EPoStCandidate{})
	encoding.RegisterIpldCborType(EPoStInfo{})
}

// EPoStInfo wraps all data needed to verify an election post proof
type EPoStInfo struct {
	_              struct{} `cbor:",toarray"`
	PoStProof      types.PoStProof
	PoStRandomness VRFPi
	Winners        []EPoStCandidate
}

// EPoStCandidate wraps the input data needed to verify an election PoSt
type EPoStCandidate struct {
	_                    struct{} `cbor:",toarray"`
	PartialTicket        []byte
	SectorID             uint64
	SectorChallengeIndex uint64
}

// NewEPoStCandidate constructs an epost candidate from data
func NewEPoStCandidate(sID uint64, pt []byte, sci uint64) EPoStCandidate {
	return EPoStCandidate{
		SectorID:             sID,
		PartialTicket:        pt,
		SectorChallengeIndex: sci,
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
		SectorNum:            abi.SectorNumber(x.SectorID),
		PartialTicket:        pt,
		Ticket:               [32]byte{}, // note: this field is ignored
		SectorChallengeIndex: x.SectorChallengeIndex,
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
		SectorID:             uint64(candidate.SectorNum),
		SectorChallengeIndex: candidate.SectorChallengeIndex,
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
