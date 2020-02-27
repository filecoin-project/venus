package consensus

import (
	"encoding/binary"
	"math/big"

	"github.com/filecoin-project/go-address"
	sector "github.com/filecoin-project/go-sectorbuilder"
	"github.com/filecoin-project/specs-actors/actors/abi"

	ffi "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/postgenerator"
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs/verification"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/util/convert"
)

// ElectionMachine generates and validates PoSt partial tickets and PoSt
// proofs.
type ElectionMachine struct{}

// GeneratePoStRandomness returns the PoStRandomness for the given epoch.
func (em ElectionMachine) GeneratePoStRandomness(ticket block.Ticket, candidateAddr address.Address, signer types.Signer, nullBlockCount uint64) ([]byte, error) {
	seedBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(seedBuf, nullBlockCount)
	buf := append(ticket.VRFProof, seedBuf[:n]...)

	signature, err := signer.SignBytes(buf[:], candidateAddr)
	if err != nil {
		return nil, err
	}
	return signature.Data, nil
}

// GenerateCandidates creates candidate partial tickets for consideration in
// block reward election
func (em ElectionMachine) GenerateCandidates(poStRand []byte, sectorInfos ffi.SortedPublicSectorInfo, ep postgenerator.PoStGenerator) ([]ffi.Candidate, error) {
	dummyFaults := []abi.SectorNumber{}
	return ep.GenerateEPostCandidates(sectorInfos, convert.To32ByteArray(poStRand), dummyFaults)
}

// GeneratePoSt creates a PoSt proof over the input PoSt candidates.  Should
// only be called on winning candidates.
func (em ElectionMachine) GeneratePoSt(allSectorInfos ffi.SortedPublicSectorInfo, challengeSeed []byte, winners []ffi.Candidate, ep postgenerator.PoStGenerator) ([]byte, error) {
	return ep.ComputeElectionPoSt(allSectorInfos, challengeSeed, winners)
}

// VerifyPoStRandomness verifies that the PoSt randomness is the result of the
// candidate signing the ticket.
func (em ElectionMachine) VerifyPoStRandomness(rand block.VRFPi, ticket block.Ticket, candidateAddr address.Address, nullBlockCount uint64) error {
	seedBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(seedBuf, nullBlockCount)
	buf := append(ticket.VRFProof, seedBuf[:n]...)

	return crypto.ValidateBlsSignature(buf[:], candidateAddr, rand)
}

// CandidateWins returns true if the input candidate wins the election
func (em ElectionMachine) CandidateWins(challengeTicket []byte, sectorNum, faultNum, networkPower, sectorSize uint64) bool {
	numSectorsSampled := sector.ElectionPostChallengeCount(sectorNum, faultNum)

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
func (em ElectionMachine) VerifyPoSt(ep verification.PoStVerifier, allSectorInfos ffi.SortedPublicSectorInfo, sectorSize uint64, challengeSeed []byte, proof []byte, candidates []block.EPoStCandidate, proverAddr address.Address) (bool, error) {
	// filter down sector infos to only those referenced by candidates
	challengeCount := sector.ElectionPostChallengeCount(uint64(len(allSectorInfos.Values())), 0)

	var randomness [32]byte
	copy(randomness[:], challengeSeed)

	var proverID [32]byte
	copy(proverID[:], proverAddr.Payload())

	return ep.VerifyPoSt(sectorSize, allSectorInfos, randomness, challengeCount, proof, block.ToFFICandidates(candidates...), proverID)
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
		VRFProof: vrfPi.Data,
	}, nil
}

// ValidateTicket verifies that the ticket's proof of randomness and delay are
// valid with respect to its parent.
func (tm TicketMachine) ValidateTicket(parent, ticket block.Ticket, signerAddr address.Address) error {
	return crypto.ValidateBlsSignature(parent.VRFProof[:], signerAddr, ticket.VRFProof)
}
