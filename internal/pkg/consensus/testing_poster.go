package consensus

import (
	"context"

	"github.com/filecoin-project/specs-actors/actors/abi"

	ffi "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/constants"
	"github.com/filecoin-project/go-filecoin/internal/pkg/postgenerator"
	"github.com/filecoin-project/go-filecoin/internal/pkg/util/hasher"
)

// FakePoSter generates and verifies election PoSts
type FakePoSter struct{}

func (ep *FakePoSter) VerifyElectionPost(context.Context, abi.PoStVerifyInfo) (bool, error) {
	return true, nil
}

var _ EPoStVerifier = new(FakePoSter)
var _ postgenerator.PoStGenerator = new(FakePoSter)

// ComputeElectionPoSt returns an election post proving that the partial
// tickets are linked to the sector commitments.
func (ep *FakePoSter) ComputeElectionPoSt(sectorInfo []abi.SectorInfo, challengeSeed abi.PoStRandomness, winners []abi.PoStCandidate) ([]abi.PoStProof, error) {
	fakePoSt := make([]byte, 1)
	fakePoSt[0] = 0xe
	return []abi.PoStProof{{
		RegisteredProof: constants.DevRegisteredPoStProof,
		ProofBytes:      fakePoSt,
	}}, nil
}

// GenerateEPostCandidates generates election post candidates
func (ep *FakePoSter) GenerateEPostCandidates(sectorInfos []abi.SectorInfo, challengeSeed abi.PoStRandomness, faults []abi.SectorNumber) ([]ffi.PoStCandidateWithTicket, error) {
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
