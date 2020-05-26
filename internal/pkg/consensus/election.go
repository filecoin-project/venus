package consensus

import (
	"context"
	"fmt"

	address "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/sector-storage/ffiwrapper"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	acrypto "github.com/filecoin-project/specs-actors/actors/crypto"
	"github.com/minio/blake2b-simd"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/drand"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/postgenerator"
	"github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// Interface to PoSt verification.
type EPoStVerifier interface {
	// VerifyWinningPoSt verifies an election PoSt.
	VerifyWinningPoSt(ctx context.Context, post abi.WinningPoStVerifyInfo) (bool, error)
	GenerateWinningPoStSectorChallenge(ctx context.Context, proofType abi.RegisteredProof, minerID abi.ActorID, randomness abi.PoStRandomness, eligibleSectorCount uint64) ([]uint64, error)
}

type SectorsStateView interface {
	MinerSectorConfiguration(ctx context.Context, maddr address.Address) (*state.MinerSectorConfiguration, error)
	MinerSectorStates(ctx context.Context, maddr address.Address) (*state.MinerSectorStates, error)
	MinerGetSector(ctx context.Context, maddr address.Address, sectorNum abi.SectorNumber) (*miner.SectorOnChainInfo, bool, error)
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
func (em ElectionMachine) GenerateWinningPoSt(ctx context.Context, entry *drand.Entry, epoch abi.ChainEpoch, ep postgenerator.PoStGenerator, maddr address.Address, sectors SectorsStateView) ([]block.PoStProof, error) {
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

	challengedSectorInfos, err := computeWinningPoStSectorChallenges(ctx, sectors, maddr, poStRandomness)
	if err != nil {
		return nil, err
	}

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
func (em ElectionMachine) VerifyWinningPoSt(ctx context.Context, ep EPoStVerifier, seedEntry *drand.Entry, epoch abi.ChainEpoch, proofs []block.PoStProof, mIDAddr address.Address, sectors SectorsStateView) (bool, error) {
	if len(proofs) == 0 {
		return false, nil
	}

	entropy, err := encoding.Encode(mIDAddr)
	if err != nil {
		return false, err
	}

	seed := blake2b.Sum256(seedEntry.Data)
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

	challengedSectorInfos, err := computeWinningPoStSectorChallenges(ctx, sectors, mIDAddr, poStRandomness)
	if err != nil {
		return false, err
	}

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
		ChallengedSectors: challengedSectorInfos,
		Prover:            minerID,
	}
	return ep.VerifyWinningPoSt(ctx, verifyInfo)
}

// Loads infos for sectors challenged by a Winning PoSt.
func computeWinningPoStSectorChallenges(ctx context.Context, sectors SectorsStateView, mIDAddr address.Address, poStRandomness abi.PoStRandomness) ([]abi.SectorInfo, error) {
	provingSet, err := computeProvingSet(ctx, sectors, mIDAddr)
	if err != nil {
		return nil, err
	}
	sectorCount, err := provingSet.Count()
	if err != nil {
		return nil, err
	}

	conf, err := sectors.MinerSectorConfiguration(ctx, mIDAddr)
	if err != nil {
		return nil, err
	}
	rp, err := conf.SealProofType.RegisteredWinningPoStProof()
	if err != nil {
		return nil, err
	}

	minerIDuint64, err := address.IDFromAddress(mIDAddr)
	if err != nil {
		return nil, err
	}
	minerID := abi.ActorID(minerIDuint64)

	challengeIndexes, err := ffiwrapper.ProofVerifier.GenerateWinningPoStSectorChallenge(ctx, rp, minerID, poStRandomness, sectorCount)
	if err != nil {
		return nil, err
	}
	challengedSectorInfos, err := loadChallengedSectors(ctx, sectors, mIDAddr, provingSet, challengeIndexes)
	if err != nil {
		return nil, err
	}
	return challengedSectorInfos, nil
}

// Computes the set of sectors that may be challenged by Winning PoSt for a miner.
func computeProvingSet(ctx context.Context, sectors SectorsStateView, maddr address.Address) (*abi.BitField, error) {
	sectorStates, err := sectors.MinerSectorStates(ctx, maddr)
	if err != nil {
		return nil, err
	}

	pset, err := abi.BitFieldUnion(sectorStates.Deadlines...)
	if err != nil {
		return nil, err
	}

	// Exclude sectors declared faulty.
	// Recoveries are a subset of faults, so not needed explicitly here.
	pset, err = bitfield.SubtractBitField(pset, sectorStates.Faults)
	if err != nil {
		return nil, err
	}

	// Include new sectors.
	// This is to replicate existing incorrect behaviour in Lotus.
	// https://github.com/filecoin-project/go-filecoin/issues/4141
	pset, err = bitfield.MergeBitFields(pset, sectorStates.NewSectors)
	return pset, err
}

func loadChallengedSectors(ctx context.Context, sectors SectorsStateView, maddr address.Address, provingSet *abi.BitField, challengeIndexes []uint64) ([]abi.SectorInfo, error) {
	challengedSectorInfos := make([]abi.SectorInfo, len(challengeIndexes))
	for i, ci := range challengeIndexes {
		// TODO: replace Slice()+First() with provingSet.Get(ci) when it exists.
		sectorNums, err := provingSet.Slice(ci, 1)
		if err != nil {
			return nil, err
		}
		sectorNum, err := sectorNums.First()
		if err != nil {
			return nil, err
		}
		si, found, err := sectors.MinerGetSector(ctx, maddr, abi.SectorNumber(sectorNum))
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("no sector %d challenging %d", sectorNum, ci)
		}
		challengedSectorInfos[i] = abi.SectorInfo{
			RegisteredProof: si.Info.RegisteredProof,
			SectorNumber:    si.Info.SectorNumber,
			SealedCID:       si.Info.SealedCID,
		}
	}
	return challengedSectorInfos, nil
}

func electionVRFRandomness(entry *drand.Entry, miner address.Address, epoch abi.ChainEpoch) (abi.Randomness, error) {
	entropy, err := encoding.Encode(miner)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to encode entropy")
	}
	seed := blake2b.Sum256(entry.Data)
	return crypto.BlendEntropy(acrypto.DomainSeparationTag_ElectionProofProduction, seed[:], epoch, entropy)
}
