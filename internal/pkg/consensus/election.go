package consensus

import (
	"bytes"
	"context"

	address "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/sector-storage/ffiwrapper"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	acrypto "github.com/filecoin-project/specs-actors/actors/crypto"
	"github.com/minio/blake2b-simd"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/drand"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/postgenerator"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// Interface to PoSt verification.
type EPoStVerifier interface {
	// VerifyWinningPoSt verifies an election PoSt.
	VerifyWinningPoSt(ctx context.Context, post abi.WinningPoStVerifyInfo) (bool, error)
	GenerateWinningPoStSectorChallenge(ctx context.Context, proofType abi.RegisteredProof, minerID abi.ActorID, randomness abi.PoStRandomness, eligibleSectorCount uint64) ([]uint64, error)
}

// ElectionMachine generates and validates PoSt partial tickets and PoSt proofs.
type ElectionMachine struct{}

func NewElectionMachine(_ ChainRandomness) *ElectionMachine {
	return &ElectionMachine{}
}

func (em ElectionMachine) GenerateElectionProof(ctx context.Context, entry *drand.Entry,
	epoch abi.ChainEpoch, miner address.Address, worker address.Address, signer types.Signer) (crypto.VRFPi, error) {
	randomness, err := electionVRFRandomness(entry, miner, epoch)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate election randomness randomness")
	}
	vrfProof, err := signer.SignBytes(ctx, randomness, worker)
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign election post randomness")
	}
	return vrfProof.Data, nil
}

// GenerateWinningPoSt creates a PoSt proof over the input miner ID and sector infos.
func (em ElectionMachine) GenerateWinningPoSt(ctx context.Context, allSectorInfos []abi.SectorInfo, entry *drand.Entry, epoch abi.ChainEpoch, ep postgenerator.PoStGenerator, maddr address.Address) ([]block.PoStProof, error) {
	entropy, err := encoding.Encode(maddr)
	if err != nil {
		return nil, err
	}

	seed := blake2b.Sum256(entry.Data)
	randomness, err := crypto.BlendEntropy(acrypto.DomainSeparationTag_WinningPoStChallengeSeed, seed[:], epoch, entropy)

	if err != nil {
		return nil, err
	}
	poStRandomness := abi.PoStRandomness(randomness)

	minerIDuint64, err := address.IDFromAddress(maddr)
	if err != nil {
		return nil, err
	}
	minerID := abi.ActorID(minerIDuint64)
	rp, err := allSectorInfos[0].RegisteredProof.RegisteredWinningPoStProof()
	if err != nil {
		return nil, err
	}
	challengeIndexes, err := ffiwrapper.ProofVerifier.GenerateWinningPoStSectorChallenge(ctx, rp, minerID, poStRandomness, uint64(len(allSectorInfos)))
	if err != nil {
		return nil, err
	}
	challengedSectorInfos := filterSectorInfosByIndex(allSectorInfos, challengeIndexes)

	posts, err := ep.GenerateWinningPoSt(ctx, minerID, challengedSectorInfos, poStRandomness)
	if err != nil {
		return nil, err
	}

	return block.FromABIPoStProofs(posts...), nil
}

func (em ElectionMachine) VerifyElectionProof(_ context.Context, entry *drand.Entry, epoch abi.ChainEpoch, miner address.Address, workerSigner address.Address, vrfProof crypto.VRFPi) error {
	randomness, err := electionVRFRandomness(entry, miner, epoch)
	if err != nil {
		return errors.Wrap(err, "failed to reproduce election randomness")
	}

	return crypto.ValidateBlsSignature(randomness, workerSigner, vrfProof)
}

// IsWinner returns true if the input challengeTicket wins the election
func (em ElectionMachine) IsWinner(challengeTicket []byte, minerPower, networkPower abi.StoragePower) bool {
	// (ChallengeTicket / MaxChallengeTicket) < ExpectedLeadersPerEpoch * (MinerPower / NetworkPower)
	// ->
	// ChallengeTicket * NetworkPower < ExpectedLeadersPerEpoch * MinerPower * MaxChallengeTicket

	lhs := big.PositiveFromUnsignedBytes(challengeTicket[:])
	lhs = big.Mul(lhs, networkPower)

	rhs := big.Lsh(minerPower, challengeBits)
	rhs = big.Mul(rhs, big.NewInt(expectedLeadersPerEpoch))

	return big.Cmp(lhs, rhs) < 0
}

// VerifyWinningPoSt verifies a Winning PoSt proof.
func (em ElectionMachine) VerifyWinningPoSt(ctx context.Context, ep EPoStVerifier, allSectorInfos []abi.SectorInfo, entry *drand.Entry, epoch abi.ChainEpoch, proofs []block.PoStProof, mIDAddr address.Address) (bool, error) {
	if len(proofs) == 0 {
		return false, nil
	}

	entropy, err := encoding.Encode(mIDAddr)
	if err != nil {
		return false, err
	}

	seed := blake2b.Sum256(entry.Data)
	randomness, err := crypto.BlendEntropy(acrypto.DomainSeparationTag_WinningPoStChallengeSeed, seed[:], epoch, entropy)
	if err != nil {
		return false, err
	}
	poStRandomness := abi.PoStRandomness(randomness)

	minerIDuint64, err := address.IDFromAddress(mIDAddr)
	if err != nil {
		return false, err
	}
	minerID := abi.ActorID(minerIDuint64)

	// Gather sector inputs for each proof
	rp, err := allSectorInfos[0].RegisteredProof.RegisteredWinningPoStProof()
	if err != nil {
		return false, err
	}
	challengeIndexes, err := ep.GenerateWinningPoStSectorChallenge(ctx, rp, minerID, poStRandomness, uint64(len(allSectorInfos)))
	if err != nil {
		return false, err
	}
	challengedSectorsInfos := filterSectorInfosByIndex(allSectorInfos, challengeIndexes)

	proofsPrime := make([]abi.PoStProof, len(proofs))
	for idx := range proofsPrime {
		proofsPrime[idx] = abi.PoStProof{
			RegisteredProof: proofs[idx].RegisteredProof,
			ProofBytes:      proofs[idx].ProofBytes,
		}
	}

	verifyInfo := abi.WinningPoStVerifyInfo{
		Randomness:        poStRandomness,
		Proofs:            proofsPrime,
		ChallengedSectors: challengedSectorsInfos,
		Prover:            minerID,
	}
	return ep.VerifyWinningPoSt(ctx, verifyInfo)
}

type ChainSampler interface {
	SampleTicket(ctx context.Context, head block.TipSetKey, epoch abi.ChainEpoch) (block.Ticket, error)
}

// TicketMachine uses a VRF and VDF to generate deterministic, unpredictable
// and time delayed tickets and validates these tickets.
type TicketMachine struct {
	sampler ChainSampler
}

func NewTicketMachine(sampler ChainSampler) *TicketMachine {
	return &TicketMachine{sampler: sampler}
}

// MakeTicket creates a new ticket from a chain and target epoch by running a verifiable
// randomness function on the prior ticket.
func (tm TicketMachine) MakeTicket(ctx context.Context, base block.TipSetKey, epoch abi.ChainEpoch, miner address.Address, entry *drand.Entry, newPeriod bool, worker address.Address, signer types.Signer) (block.Ticket, error) {
	randomness, err := tm.ticketVRFRandomness(ctx, base, entry, newPeriod, miner, epoch)
	if err != nil {
		return block.Ticket{}, errors.Wrap(err, "failed to generate ticket randomness")
	}
	vrfProof, err := signer.SignBytes(ctx, randomness, worker)
	if err != nil {
		return block.Ticket{}, errors.Wrap(err, "failed to sign election post randomness")
	}
	return block.Ticket{
		VRFProof: vrfProof.Data,
	}, nil
}

// IsValidTicket verifies that the ticket's proof of randomness is valid with respect to its parent.
func (tm TicketMachine) IsValidTicket(ctx context.Context, base block.TipSetKey, entry *drand.Entry, newPeriod bool,
	epoch abi.ChainEpoch, miner address.Address, workerSigner address.Address, ticket block.Ticket) error {
	randomness, err := tm.ticketVRFRandomness(ctx, base, entry, newPeriod, miner, epoch)
	if err != nil {
		return errors.Wrap(err, "failed to generate ticket randomness")
	}

	return crypto.ValidateBlsSignature(randomness, workerSigner, ticket.VRFProof)
}

func (tm TicketMachine) ticketVRFRandomness(ctx context.Context, base block.TipSetKey, entry *drand.Entry, newPeriod bool, miner address.Address, epoch abi.ChainEpoch) (abi.Randomness, error) {
	entropyBuf := bytes.Buffer{}
	minerEntropy, err := encoding.Encode(miner)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to encode miner entropy")
	}
	_, err = entropyBuf.Write(minerEntropy)
	if err != nil {
		return nil, err
	}
	if !newPeriod { // resample previous ticket and add to entropy
		ticket, err := tm.sampler.SampleTicket(ctx, base, epoch)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to sample previous ticket")
		}
		_, err = entropyBuf.Write(ticket.VRFProof)
		if err != nil {
			return nil, err
		}
	}
	seed := blake2b.Sum256(entry.Data)
	return crypto.BlendEntropy(acrypto.DomainSeparationTag_TicketProduction, seed[:], epoch, entropyBuf.Bytes())
}

func electionVRFRandomness(entry *drand.Entry, miner address.Address, epoch abi.ChainEpoch) (abi.Randomness, error) {
	entropy, err := encoding.Encode(miner)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to encode entropy")
	}
	seed := blake2b.Sum256(entry.Data)
	return crypto.BlendEntropy(acrypto.DomainSeparationTag_ElectionProofProduction, seed[:], epoch, entropy)
}

func filterSectorInfosByIndex(allSectorInfos []abi.SectorInfo, challengeIDs []uint64) []abi.SectorInfo {
	idSet := make(map[uint64]struct{})
	for _, id := range challengeIDs {
		idSet[id] = struct{}{}
	}

	var filteredSectorInfos []abi.SectorInfo
	for i, si := range allSectorInfos {
		if _, ok := idSet[uint64(i)]; ok {
			filteredSectorInfos = append(filteredSectorInfos, si)
		}
	}
	return filteredSectorInfos
}
