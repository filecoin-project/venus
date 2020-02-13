package proofs

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/postgenerator"
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs/verification"
	"github.com/filecoin-project/go-filecoin/internal/pkg/util/convert"
	"github.com/filecoin-project/specs-actors/actors/abi"

	ffi "github.com/filecoin-project/filecoin-ffi"

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
func (ep *ElectionPoster) VerifyPoSt(sectorSize uint64, sectorInfo ffi.SortedPublicSectorInfo, challengeSeed [32]byte, challengeCount uint64, proof []byte, candidates []ffi.Candidate, proverID [32]byte) (bool, error) {
	return true, nil
}

// ComputeElectionPoSt returns an election post proving that the partial
// tickets are linked to the sector commitments.
func (ep *ElectionPoster) ComputeElectionPoSt(sectorInfo ffi.SortedPublicSectorInfo, challengeSeed []byte, winners []ffi.Candidate) ([]byte, error) {
	fakePoSt := make([]byte, 1)
	fakePoSt[0] = 0xe
	return fakePoSt, nil
}

// GenerateEPostCandidates generates election post candidates
func (ep *ElectionPoster) GenerateEPostCandidates(sectorInfo ffi.SortedPublicSectorInfo, challengeSeed [32]byte, faults []abi.SectorNumber) ([]ffi.Candidate, error) {
	// Current fake behavior: generate one partial ticket per sector,
	// each partial ticket is the hash of the challengeSeed and sectorID
	var candidates []ffi.Candidate
	hasher := hasher.NewHasher()
	for i, si := range sectorInfo.Values() {
		hasher.Int(uint64(si.SectorNum))
		hasher.Bytes(challengeSeed[:])
		nextCandidate := ffi.Candidate{
			SectorNum:            si.SectorNum,
			PartialTicket:        convert.To32ByteArray(hasher.Hash()),
			Ticket:               [32]byte{},
			SectorChallengeIndex: uint64(i), //fake value of sector idx
		}
		candidates = append(candidates, nextCandidate)
	}

	return candidates, nil
}
