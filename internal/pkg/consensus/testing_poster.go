package consensus

import (
	"context"

	"github.com/filecoin-project/specs-actors/actors/abi"

	ffi "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/constants"
	"github.com/filecoin-project/go-filecoin/internal/pkg/postgenerator"
	"github.com/filecoin-project/go-filecoin/internal/pkg/util/hasher"
)

// TestElectionPoster generates and verifies electoin PoSts
type TestElectionPoster struct{}

var _ EPoStVerifier = new(TestElectionPoster)
var _ postgenerator.PoStGenerator = new(TestElectionPoster)

// VerifyElectionPost returns the validity of the input PoSt proof
func (ep *TestElectionPoster) VerifyElectionPost(_ context.Context, _ abi.PoStVerifyInfo) (bool, error) {
	return true, nil
}

// ComputeElectionPoSt returns an election post proving that the partial
// tickets are linked to the sector commitments.
func (ep *TestElectionPoster) ComputeElectionPoSt(sectorInfo []abi.SectorInfo, challengeSeed abi.PoStRandomness, winners []abi.PoStCandidate) ([]abi.PoStProof, error) {
	fakePoSt := make([]byte, 1)
	fakePoSt[0] = 0xe
	return []abi.PoStProof{{
		RegisteredProof: constants.DevRegisteredPoStProof,
		ProofBytes:      fakePoSt,
	}}, nil
}

// GenerateEPostCandidates generates election post candidates
func (ep *TestElectionPoster) GenerateEPostCandidates(sectorInfos []abi.SectorInfo, challengeSeed abi.PoStRandomness, faults []abi.SectorNumber) ([]ffi.PoStCandidateWithTicket, error) {
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
