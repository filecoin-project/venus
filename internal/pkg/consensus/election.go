package consensus

import (
	"context"
	"encoding/binary"
	"math/big"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	sector "github.com/filecoin-project/go-sectorbuilder"
)

// ElectionMachine generates and validates PoSt partial tickets and PoSt
// proofs.
type ElectionMachine struct{}

// GeneratePoStRandomness returns the PoStRandomness for the given epoch.
func (em ElectionMachine) GeneratePoStRandomness(ticket block.Ticket, candidateAddr address.Address, signer types.Signer, nullBlockCount uint64) ([]byte, error) {
	seedBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(seedBuf, nullBlockCount)
	buf := append(ticket.VRFProof, seedBuf[:n]...)

	return signer.SignBytes(buf[:], candidateAddr)
}

// GenerateCandidates creates candidate partial tickets for consideration in
// block reward election
func (em ElectionMachine) GenerateCandidates(poStRand []byte, sectorInfos sector.SortedSectorInfo, ep *proofs.ElectionPoster) ([]*proofs.EPoStCandidate, error) {
	dummyFaults := []uint64{}
	return ep.GenerateEPostCandidates(sectorInfos, poStRand, dummyFaults)
}

// GeneratePoSt creates a PoSt proof over the input PoSt candidates.  Should
// only be called on winning candidates.
func (em ElectionMachine) GeneratePoSt(allSectorInfos sector.SortedSectorInfo, challengeSeed []byte, winners []*proofs.EPoStCandidate, ep *proofs.ElectionPoster) ([]byte, error) {
	winnerSectorInfos := filterSectorInfosByCandidates(allSectorInfos, winners)
	return ep.ComputeElectionPoSt(winnerSectorInfos, challengeSeed, winners)
}

// VerifyPoStRandomness verifies that the PoSt randomness is the result of the
// candidate signing the ticket.
func (em ElectionMachine) VerifyPoStRandomness(rand block.VRFPi, ticket block.Ticket, candidateAddr address.Address, nullBlockCount uint64) bool {
	seedBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(seedBuf, nullBlockCount)
	buf := append(ticket.VRFProof, seedBuf[:n]...)

	vrfPi := types.Signature(rand)
	return types.IsValidSignature(buf[:], candidateAddr, vrfPi)
}

// CandidateWins returns true if the input candidate wins the election
func (em ElectionMachine) CandidateWins(challengeTicket []byte, ep *proofs.ElectionPoster, sectorNum, faultNum, networkPower, sectorSize uint64) bool {
	numSectorsSampled := ep.ElectionPostChallengeCount(sectorNum, faultNum)

	lhs := new(big.Int).SetBytes(challengeTicket[:])
	lhs = lhs.Mul(lhs, big.NewInt(int64(networkPower)))
	lhs = lhs.Mul(lhs, big.NewInt(int64(numSectorsSampled)))

	// sectorPower * 2^len(H)
	rhs := new(big.Int).Lsh(big.NewInt(int64(sectorSize)), challengeBits)
	rhs = rhs.Mul(rhs, big.NewInt(int64(sectorNum)))
	rhs = rhs.Mul(rhs, big.NewInt(expectedLeadersPerEpoch))

	// lhs < rhs?
	// (challengeTicket / maxChallengeTicket) < expectedLeadersPerEpoch * (effective miner power) / networkPower
	// effective miner power = sectorSize * numberSectors / numSectorsSampled
	return lhs.Cmp(rhs) == -1
}

// VerifyPoSt verifies a PoSt proof.
func (em ElectionMachine) VerifyPoSt(ctx context.Context, ep *proofs.ElectionPoster, allSectorInfos sector.SortedSectorInfo, sectorSize uint64, challengeSeed []byte, proof []byte, candidates []*proofs.EPoStCandidate, proverID address.Address) (bool, error) {
	// filter down sector infos to only those referenced by candidates
	return ep.VerifyElectionPost(
		ctx,
		sectorSize,
		filterSectorInfosByCandidates(allSectorInfos, candidates),
		challengeSeed,
		proof,
		candidates,
		proverID,
	)
}

func filterSectorInfosByCandidates(allSectorInfos sector.SortedSectorInfo, candidates []*proofs.EPoStCandidate) sector.SortedSectorInfo {
	candidateSectorID := make(map[uint64]struct{})
	for _, candidate := range candidates {
		candidateSectorID[candidate.SectorID] = struct{}{}
	}
	var candidateSectorInfos []sector.SectorInfo
	for _, si := range allSectorInfos.Values() {
		if _, ok := candidateSectorID[si.SectorID]; ok {
			candidateSectorInfos = append(candidateSectorInfos, si)
		}
	}
	return sector.NewSortedSectorInfo(candidateSectorInfos...)
}

// TicketMachine uses a VRF and VDF to generate deterministic, unpredictable
// and time delayed tickets and validates these tickets.
type TicketMachine struct{}

// NextTicket creates a new ticket from a parent ticket by running a verifiable
// randomness function on the parent.
func (tm TicketMachine) NextTicket(parent block.Ticket, signerAddr address.Address, signer types.Signer) (block.Ticket, error) {
	vrfPi, err := signer.SignBytes(parent.VRFProof[:], signerAddr)
	if err != nil {
		return block.Ticket{}, err
	}

	return block.Ticket{
		VRFProof: block.VRFPi(vrfPi),
	}, nil
}

// IsValidTicket verifies that the ticket's proof of randomness and delay are
// valid with respect to its parent.
func (tm TicketMachine) IsValidTicket(parent, ticket block.Ticket, signerAddr address.Address) bool {
	vrfPi := types.Signature(ticket.VRFProof)
	return types.IsValidSignature(parent.VRFProof[:], signerAddr, vrfPi)
}
