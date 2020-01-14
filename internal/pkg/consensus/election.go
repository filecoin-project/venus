package consensus

import (
	"context"
	"encoding/binary"
	"math/big"

	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/util/hasher"
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
func (em ElectionMachine) GeneratePoSt(sectorInfo sector.SortedSectorInfo, challengeSeed []byte, winners []*proofs.EPoStCandidate, ep *proofs.ElectionPoster) ([]byte, error) {
	return ep.ComputeElectionPoSt(sectorInfo, challengeSeed, winners)
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
func (em ElectionMachine) CandidateWins(candidate proofs.EPoStCandidate, ep *proofs.ElectionPoster, sectorNum, faultNum, networkPower, sectorSize uint64) bool {

	hasher := hasher.NewHasher()
	hasher.Bytes(candidate.PartialTicket)
	challengeTicket := hasher.Hash()
	numSectorsSampled := ep.ElectionPostChallengeCount(sectorNum, faultNum)

	lhs := new(big.Int).SetBytes(challengeTicket[:])
	lhs = lhs.Mul(lhs, big.NewInt(int64(networkPower)))
	lhs = lhs.Mul(lhs, big.NewInt(int64(numSectorsSampled)))

	// sectorPower * 2^len(H)
	rhs := new(big.Int).Lsh(big.NewInt(int64(sectorSize)), challengeBits)
	rhs = rhs.Mul(rhs, big.NewInt(int64(sectorNum)))
	rhs = rhs.Mul(rhs, big.NewInt(expectedLeadersPerEpoch))

	// lhs < rhs?
	return lhs.Cmp(rhs) == -1
}

// VerifyPoSt verifies a PoSt proof.
func (em ElectionMachine) VerifyPoSt(ctx context.Context, ep *proofs.ElectionPoster, allSectorInfos sector.SortedSectorInfo, sectorSize uint64, challengeSeed []byte, proof []byte, candidates []*proofs.EPoStCandidate, proverID address.Address) (bool, error) {
	// filter down sector infos to only those referenced by candidates
	candidateSectorID := make(map[uint64]struct{})
	for _, candidate := range candidates {
		candidateSectorID[candidate.SectorID] = struct{}{}
	}
	var candidateSectorInfos []sector.SectorInfo
	for _, si := range allSectorInfos.Values() {
		candidateSectorInfos = append(candidateSectorInfos, si)
	}

	return ep.VerifyElectionPost(
		ctx,
		sectorSize,
		sector.NewSortedSectorInfo(candidateSectorInfos...),
		challengeSeed,
		proof,
		candidates,
		proverID,
	)
}

// DeprecatedCompareElectionPower return true if the input electionProof is below the
// election victory threshold for the input miner and global power values.
func DeprecatedCompareElectionPower(electionProof block.VRFPi, minerPower *types.BytesAmount, totalPower *types.BytesAmount) bool {
	lhs := &big.Int{}
	lhs.SetBytes(electionProof)
	lhs.Mul(lhs, totalPower.BigInt())
	rhs := &big.Int{}
	rhs.Mul(minerPower.BigInt(), ticketDomain)

	return lhs.Cmp(rhs) < 0
}

// DeprecatedRunElection uses a VRF to run a secret, verifiable election with respect to
// an input ticket.
func (em ElectionMachine) DeprecatedRunElection(ticket block.Ticket, candidateAddr address.Address, signer types.Signer, nullBlockCount uint64) (block.VRFPi, error) {
	seedBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(seedBuf, nullBlockCount)
	buf := append(ticket.VRFProof, seedBuf[:n]...)

	vrfPi, err := signer.SignBytes(buf[:], candidateAddr)
	if err != nil {
		return block.VRFPi{}, err
	}

	return block.VRFPi(vrfPi), nil
}

// DeprecatedIsElectionWinner verifies that an election proof was validly generated and
// is a winner.  TODO #3418 improve state management to clean up interface.
func (em ElectionMachine) DeprecatedIsElectionWinner(ctx context.Context, ptv PowerTableView, ticket block.Ticket, nullBlockCount uint64, electionProof block.VRFPi, signingAddr, minerAddr address.Address) (bool, error) {
	seedBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(seedBuf, nullBlockCount)
	buf := append(ticket.VRFProof, seedBuf[:n]...)

	// Verify election proof is valid
	vrfPi := types.Signature(electionProof)
	if valid := types.IsValidSignature(buf[:], signingAddr, vrfPi); !valid {
		return false, nil
	}

	// Verify election proof is a winner
	totalPower, err := ptv.Total(ctx)
	if err != nil {
		return false, errors.Wrap(err, "Couldn't get totalPower")
	}

	minerPower, err := ptv.Miner(ctx, minerAddr)
	if err != nil {
		return false, errors.Wrap(err, "Couldn't get minerPower")
	}

	return DeprecatedCompareElectionPower(electionProof, minerPower, totalPower), nil
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
