package proofs

import (
	ffi "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/postgenerator"
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs/verification"
	"github.com/filecoin-project/specs-actors/actors/abi"

	"github.com/filecoin-project/go-filecoin/internal/pkg/util/hasher"
)

// SectorChallengeRatioDiv is the number of sectors per candidate partial
// ticket
const SectorChallengeRatioDiv = 25

// ElectionPoster generates and verifies electoin PoSts
// Dragons: once we have a proper eposter this type should either be
// replaced or it should be a thin wrapper around the proper eposter
type ElectionPoster struct{}

var _ verification.PoStVerifier = new(ElectionPoster)
var _ postgenerator.PoStGenerator = new(ElectionPoster)

// VerifyPoSt returns the validity of the input PoSt proof
func (ep *ElectionPoster) VerifyPoSt(_ abi.PoStVerifyInfo) (bool, error) {
	return true, nil
}

// ComputeElectionPoSt returns an election post proving that the partial
// tickets are linked to the sector commitments.
func (ep *ElectionPoster) ComputeElectionPoSt(sectorInfo []abi.SectorInfo, challengeSeed abi.PoStRandomness, winners []abi.PoStCandidate) ([]abi.PoStProof, error) {
	fakePoSt := make([]byte, 1)
	fakePoSt[0] = 0xe
	return []abi.PoStProof{{
		RegisteredProof: abi.RegisteredProof_StackedDRG2KiBPoSt,
		ProofBytes:      fakePoSt,
	}}, nil
}

// GenerateEPostCandidates generates election post candidates
func (ep *ElectionPoster) GenerateEPostCandidates(sectorInfos []abi.SectorInfo, challengeSeed abi.PoStRandomness, faults []abi.SectorNumber) ([]ffi.PoStCandidateWithTicket, error) {
	// Current fake behavior: generate one partial ticket per sector,
	// each partial ticket is the hash of the challengeSeed and sectorID
	var candidatesWithTickets []ffi.PoStCandidateWithTicket
	hasher := hasher.NewHasher()
	for i, si := range sectorInfos {
		hasher.Int(uint64(si.SectorNumber))
		hasher.Bytes(challengeSeed[:])
		nextCandidate := abi.PoStCandidate{
			// TODO: when we actually use this in test the object must be set up with actor id
			SectorID:       abi.SectorID{Miner: abi.ActorID(1), Number: si.SectorNumber},
			PartialTicket:  hasher.Hash(),
			ChallengeIndex: int64(i), //fake value of sector idx
		}
		nextCandidateWithTicket := ffi.PoStCandidateWithTicket{
			Candidate: nextCandidate,
		}
		candidatesWithTickets = append(candidatesWithTickets, nextCandidateWithTicket)
	}

	return candidatesWithTickets, nil
}
