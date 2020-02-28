package consensus

import (
	"context"
	"math/big"

	"github.com/filecoin-project/go-address"
	sector "github.com/filecoin-project/go-sectorbuilder"
	"github.com/filecoin-project/specs-actors/actors/abi"
	acrypto "github.com/filecoin-project/specs-actors/actors/crypto"
	"github.com/pkg/errors"

	ffi "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/postgenerator"
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs/verification"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/util/convert"
)

// ElectionMachine generates and validates PoSt partial tickets and PoSt proofs.
type ElectionMachine struct {
	chain ChainRandomness
}

func NewElectionMachine(chain ChainRandomness) *ElectionMachine {
	return &ElectionMachine{chain: chain}
}

// GenerateEPoStVrfProof computes the Election PoSt challenge seed for a target epoch after a base tipset.
func (em ElectionMachine) GenerateEPoStVrfProof(ctx context.Context, base block.TipSetKey,
	epoch abi.ChainEpoch, miner address.Address, worker address.Address, signer types.Signer) (block.VRFPi, error) {
	return computeVrf(ctx, em.chain, base, epoch, acrypto.DomainSeparationTag_ElectionPoStChallengeSeed, miner, signer, worker)
}

// GenerateCandidates creates candidate partial tickets for consideration in
// block reward election
func (em ElectionMachine) GenerateCandidates(poStRand []byte, sectorInfos ffi.SortedPublicSectorInfo, ep postgenerator.PoStGenerator) ([]ffi.Candidate, error) {
	dummyFaults := []abi.SectorNumber{}
	return ep.GenerateEPostCandidates(sectorInfos, convert.To32ByteArray(poStRand), dummyFaults)
}

// GenerateEPoSt creates a PoSt proof over the input PoSt candidates.  Should
// only be called on winning candidates.
func (em ElectionMachine) GenerateEPoSt(allSectorInfos ffi.SortedPublicSectorInfo, challengeSeed []byte, winners []ffi.Candidate, ep postgenerator.PoStGenerator) ([]byte, error) {
	return ep.ComputeElectionPoSt(allSectorInfos, challengeSeed, winners)
}

// VerifyEPoStVrfProof verifies that the PoSt randomness is the result of the
// candidate signing the ticket.
func (em ElectionMachine) VerifyEPoStVrfProof(ctx context.Context, base block.TipSetKey,
	epoch abi.ChainEpoch, miner address.Address, worker address.Address, vrfProof block.VRFPi) error {
	entropy, err := encoding.Encode(miner)
	if err != nil {
		return errors.Wrapf(err, "failed to encode entropy")
	}
	randomness, err := em.chain.SampleChainRandomness(ctx, base, acrypto.DomainSeparationTag_ElectionPoStChallengeSeed, epoch, entropy)
	if err != nil {
		return errors.Wrap(err, "failed to generate epost randomness")
	}

	return crypto.ValidateBlsSignature(randomness, worker, vrfProof)
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
type TicketMachine struct {
	chain ChainRandomness
}

func NewTicketMachine(chain ChainRandomness) *TicketMachine {
	return &TicketMachine{chain: chain}
}

// MakeTicket creates a new ticket from a chain and target epoch by running a verifiable
// randomness function on the prior ticket.
func (tm TicketMachine) MakeTicket(ctx context.Context, base block.TipSetKey, epoch abi.ChainEpoch, miner address.Address, worker address.Address, signer types.Signer) (block.Ticket, error) {
	vrfProof, err := computeVrf(ctx, tm.chain, base, epoch, acrypto.DomainSeparationTag_TicketProduction, miner, signer, worker)
	return block.Ticket{
		VRFProof: vrfProof,
	}, err
}

// IsValidTicket verifies that the ticket's proof of randomness is valid with respect to its parent.
func (tm TicketMachine) IsValidTicket(ctx context.Context, base block.TipSetKey,
	epoch abi.ChainEpoch, miner address.Address, worker address.Address, ticket block.Ticket) error {
	entropy, err := encoding.Encode(miner)
	if err != nil {
		return errors.Wrapf(err, "failed to encode entropy")
	}
	randomness, err := tm.chain.SampleChainRandomness(ctx, base, acrypto.DomainSeparationTag_TicketProduction, epoch, entropy)
	if err != nil {
		return errors.Wrap(err, "failed to generate epost randomness")
	}

	return crypto.ValidateBlsSignature(randomness, worker, ticket.VRFProof)
}

func computeVrf(ctx context.Context, chain ChainRandomness, base block.TipSetKey, epoch abi.ChainEpoch, tag acrypto.DomainSeparationTag,
	miner address.Address, signer types.Signer, worker address.Address) (block.VRFPi, error) {
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
