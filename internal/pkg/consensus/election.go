package consensus

import (
	"context"
	"math/big"

	address "github.com/filecoin-project/go-address"
	sector "github.com/filecoin-project/go-sectorbuilder"
	"github.com/filecoin-project/specs-actors/actors/abi"
	acrypto "github.com/filecoin-project/specs-actors/actors/crypto"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/postgenerator"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// Interface to PoSt verification.
type EPoStVerifier interface {
	// This is a sub-interface of go-sectorbuilder's Verifier interface.
	VerifyElectionPost(ctx context.Context, post abi.PoStVerifyInfo) (bool, error)
}

// ElectionMachine generates and validates PoSt partial tickets and PoSt proofs.
type ElectionMachine struct {
	chain ChainRandomness
}

func NewElectionMachine(chain ChainRandomness) *ElectionMachine {
	return &ElectionMachine{chain: chain}
}

// GenerateEPoStVrfProof computes the Election PoSt challenge seed for a target epoch after a base tipset.
func (em ElectionMachine) GenerateEPoStVrfProof(ctx context.Context, base block.TipSetKey,
	epoch abi.ChainEpoch, miner address.Address, worker address.Address, signer types.Signer) (crypto.VRFPi, error) {
	return sampleChainAndComputeVrf(ctx, em.chain, base, epoch, acrypto.DomainSeparationTag_ElectionPoStChallengeSeed, miner, signer, worker)
}

// GenerateCandidates creates candidate partial tickets for consideration in
// block reward election
func (em ElectionMachine) GenerateCandidates(poStRand abi.PoStRandomness, sectorInfos []abi.SectorInfo, ep postgenerator.PoStGenerator) ([]abi.PoStCandidate, error) {
	dummyFaults := []abi.SectorNumber{}
	candidatesWithTicket, err := ep.GenerateEPostCandidates(sectorInfos, poStRand, dummyFaults)
	if err != nil {
		return nil, err
	}
	candidates := make([]abi.PoStCandidate, len(candidatesWithTicket))
	for i, candidateWithTicket := range candidatesWithTicket {
		candidates[i] = candidateWithTicket.Candidate
	}
	return candidates, nil
}

// GenerateEPoSt creates a PoSt proof over the input PoSt candidates.  Should
// only be called on winning candidates.
func (em ElectionMachine) GenerateEPoSt(allSectorInfos []abi.SectorInfo, challengeSeed abi.PoStRandomness, winners []abi.PoStCandidate, ep postgenerator.PoStGenerator) ([]abi.PoStProof, error) {
	return ep.ComputeElectionPoSt(allSectorInfos, challengeSeed, winners)
}

// VerifyEPoStVrfProof verifies that the PoSt randomness is the result of the
// candidate signing the ticket.
func (em ElectionMachine) VerifyEPoStVrfProof(ctx context.Context, base block.TipSetKey, epoch abi.ChainEpoch, miner address.Address, workerSigner address.Address, vrfProof abi.PoStRandomness) error {
	entropy, err := encoding.Encode(miner)
	if err != nil {
		return errors.Wrapf(err, "failed to encode entropy")
	}
	randomness, err := em.chain.SampleChainRandomness(ctx, base, acrypto.DomainSeparationTag_ElectionPoStChallengeSeed, epoch, entropy)
	if err != nil {
		return errors.Wrap(err, "failed to generate epost randomness")
	}

	return crypto.ValidateBlsSignature(randomness, workerSigner, vrfProof)
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
func (em ElectionMachine) VerifyPoSt(ctx context.Context, ep EPoStVerifier, allSectorInfos []abi.SectorInfo, challengeSeed abi.PoStRandomness, proofs []block.EPoStProof, candidates []block.EPoStCandidate, mIDAddr address.Address) (bool, error) {
	// filter down sector infos to only those referenced by candidates
	challengeCount := sector.ElectionPostChallengeCount(uint64(len(allSectorInfos)), 0)
	minerID, err := address.IDFromAddress(mIDAddr)
	if err != nil {
		return false, err
	}

	sectorNumToRegisteredProof := make(map[abi.SectorNumber]abi.RegisteredProof)
	for _, si := range allSectorInfos {
		rpp, err := si.RegisteredPoStProof()
		if err != nil {
			return false, err
		}
		sectorNumToRegisteredProof[si.SectorNumber] = rpp
	}

	// map inputs to abi.PoStVerifyInfo
	var ffiCandidates []abi.PoStCandidate
	for _, candidate := range candidates {
		c := abi.PoStCandidate{
			RegisteredProof: sectorNumToRegisteredProof[candidate.SectorID],
			PartialTicket:   candidate.PartialTicket,
			SectorID: abi.SectorID{
				Miner:  abi.ActorID(minerID),
				Number: candidate.SectorID,
			},
			ChallengeIndex: candidate.SectorChallengeIndex,
		}
		ffiCandidates = append(ffiCandidates, c)
	}

	proofsPrime := make([]abi.PoStProof, len(proofs))
	for idx := range proofsPrime {
		proofsPrime[idx] = abi.PoStProof{
			RegisteredProof: proofs[idx].RegisteredProof,
			ProofBytes:      proofs[idx].ProofBytes,
		}
	}

	poStVerifyInfo := abi.PoStVerifyInfo{
		Randomness:      challengeSeed,
		Candidates:      ffiCandidates,
		Proofs:          proofsPrime,
		EligibleSectors: allSectorInfos,
		Prover:          abi.ActorID(minerID),
		ChallengeCount:  challengeCount,
	}

	return ep.VerifyElectionPost(ctx, poStVerifyInfo)
}

// TicketMachine uses a VRF and VDF to generate deterministic, unpredictable
// and time delayed tickets and validates these tickets.
type TicketMachine struct {
	chain ChainRandomness
}

func NewTicketMachine(chain ChainRandomness) *TicketMachine {
	return &TicketMachine{chain: chain}
}

// MakeTicket creates a new ticket from a chain and target epoch by running a verifiable
// randomness function on the prior ticket.
func (tm TicketMachine) MakeTicket(ctx context.Context, base block.TipSetKey, epoch abi.ChainEpoch, miner address.Address, worker address.Address, signer types.Signer) (block.Ticket, error) {
	vrfProof, err := sampleChainAndComputeVrf(ctx, tm.chain, base, epoch, acrypto.DomainSeparationTag_TicketProduction, miner, signer, worker)
	return block.Ticket{
		VRFProof: vrfProof,
	}, err
}

// IsValidTicket verifies that the ticket's proof of randomness is valid with respect to its parent.
func (tm TicketMachine) IsValidTicket(ctx context.Context, base block.TipSetKey,
	epoch abi.ChainEpoch, miner address.Address, workerSigner address.Address, ticket block.Ticket) error {
	entropy, err := encoding.Encode(miner)
	if err != nil {
		return errors.Wrapf(err, "failed to encode entropy")
	}
	randomness, err := tm.chain.SampleChainRandomness(ctx, base, acrypto.DomainSeparationTag_TicketProduction, epoch, entropy)
	if err != nil {
		return errors.Wrap(err, "failed to generate epost randomness")
	}

	return crypto.ValidateBlsSignature(randomness, workerSigner, ticket.VRFProof)
}

func sampleChainAndComputeVrf(ctx context.Context, chain ChainRandomness, base block.TipSetKey, epoch abi.ChainEpoch, tag acrypto.DomainSeparationTag,
	miner address.Address, signer types.Signer, worker address.Address) (crypto.VRFPi, error) {
	entropy, err := encoding.Encode(miner)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to encode entropy")
	}
	randomness, err := chain.SampleChainRandomness(ctx, base, tag, epoch, entropy)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate epost randomness")
	}
	vrfProof, err := signer.SignBytes(randomness, worker)
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign election post randomness")
	}
	return vrfProof.Data, nil
}
