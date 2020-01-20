package block

import (
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
