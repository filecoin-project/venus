package consensus

import (
	"context"
	"math/big"

	address "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
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
	// VerifyElectionPost verifies an election PoSt.
	VerifyElectionPost(ctx context.Context, post abi.PoStVerifyInfo) (bool, error)
}

// ElectionMachine generates and validates PoSt partial tickets and PoSt proofs.
type ElectionMachine struct{}

func NewElectionMachine(chain ChainRandomness) *ElectionMachine {
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
func (em ElectionMachine) GenerateWinningPoSt(allSectorInfos []abi.SectorInfo, entry *drand.Entry, epoch abi.ChainEpoch, ep postgenerator.PoStGenerator, maddr address.Address) ([]abi.PoStProof, error) {
	minerID, err := address.IDFromAddress(maddr)
	if err != nil {
		return nil, err
	}
	_ = minerID

	seed := blake2b.Sum256(entry.Signature)
	randomness, err := crypto.BlendEntropy(acrypto.DomainSeparationTag_ElectionPoStChallengeSeed, seed[:], epoch, []byte{})
	if err != nil {
		return nil, err
	}
	_ = randomness

	// challengeIndexes, err := ep.GenerateWinningPoStSectorChallenge(proofs[0].RegisteredProof, minerID, randomness, len(allPrivateSectorInfos))
	// if err != nil {
	// 	return nil, err
	// }
	// challengedSectorInfos := filterSectorInfosByIndex(allSectorInfos, challengeIndexes)
	// return ep.GenerateWinningPoSt(minerID, challengedSectorInfos, randomness)

	return nil, nil
}

func (em ElectionMachine) VerifyElectionProof(ctx context.Context, entry *drand.Entry, epoch abi.ChainEpoch, miner address.Address, workerSigner address.Address, vrfProof crypto.VRFPi) error {
	randomness, err := electionVRFRandomness(entry, miner, epoch)
	if err != nil {
		return errors.Wrap(err, "failed to reproduce election randomness")
	}

	return crypto.ValidateBlsSignature(randomness, workerSigner, vrfProof)
}

// IsWinner returns true if the input challengeTicket wins the election
func (em ElectionMachine) IsWinner(challengeTicket []byte, sectorNum, networkPower, sectorSize uint64) bool {
	lhs := new(big.Int).SetBytes(challengeTicket[:])
	lhs = lhs.Mul(lhs, big.NewInt(int64(networkPower)))

	rhs := new(big.Int).Lsh(big.NewInt(int64(sectorSize)), challengeBits)
	rhs = rhs.Mul(rhs, big.NewInt(int64(sectorNum)))
	rhs = rhs.Mul(rhs, big.NewInt(expectedLeadersPerEpoch))

	// lhs < rhs?
	// (ChallengeTicket / MaxChallengeTicket) < ExpectedLeadersPerEpoch *  (MinerPower / NetworkPower)
	return lhs.Cmp(rhs) == -1
}

// VerifyWinningPoSt verifies a Winning PoSt proof.
func (em ElectionMachine) VerifyWinningPoSt(ctx context.Context, ep EPoStVerifier, allSectorInfos []abi.SectorInfo, entry *drand.Entry, epoch abi.ChainEpoch, proofs []block.PoStProof, mIDAddr address.Address) (bool, error) {
	if len(proofs) == 0 {
		return false, nil
	}

	seed := blake2b.Sum256(entry.Signature)
	randomness, err := crypto.BlendEntropy(acrypto.DomainSeparationTag_ElectionPoStChallengeSeed, seed[:], epoch, []byte{})
	if err != nil {
		return false, err
	}
	_ = randomness

	minerID, err := address.IDFromAddress(mIDAddr)
	if err != nil {
		return false, err
	}
	_ = minerID

	// Gather sector inputs for each proof
	// challengeIndexes, err := ep.GenerateWinningPoStSectorChallenge(proofs[0].RegisteredProof, minerID, randomness, len(allSectorInfos))
	// if err != nil {
	// 	return false, nil
	// }
	// challengedSectorsInfos := filterSectorInfosByIndex(allSectorInfos, challengeIndexes)

	proofsPrime := make([]abi.PoStProof, len(proofs))
	for idx := range proofsPrime {
		proofsPrime[idx] = abi.PoStProof{
			RegisteredProof: proofs[idx].RegisteredProof,
			ProofBytes:      proofs[idx].ProofBytes,
		}
	}

	return true, nil
	// verifyInfo := abi.WinningPoStVerifyInfo{
	// 	Randomness:      randomness,
	// 	Proofs:          proofsPrime,
	// 	EligibleSectors: challengedSectorInfos,
	// 	Prover:          abi.ActorID(minerID),
	// }
	//	return ep.VerifyWinningPost(ctx, poStVerifyInfo)
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
	entropy, err := encoding.Encode(miner)
	if err != nil {
		return block.Ticket{}, errors.Wrapf(err, "failed to encode entropy")
	}
	randomness, err := tm.chain.SampleChainRandomness(ctx, base, acrypto.DomainSeparationTag_TicketProduction, epoch, entropy)
	if err != nil {
		return block.Ticket{}, errors.Wrap(err, "failed to generate epost randomness")
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

func electionVRFRandomness(entry *drand.Entry, miner address.Address, epoch abi.ChainEpoch) (abi.Randomness, error) {
	entropy, err := encoding.Encode(miner)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to encode entropy")
	}
	seed := blake2b.Sum256(entry.Signature)
	return crypto.BlendEntropy(acrypto.DomainSeparationTag_ElectionPoStChallengeSeed, seed[:], epoch, entropy)
}

func filterSectorInfosByIndex(allSectorInfos []abi.SectorInfo, challengeIDs []uint64) []abi.SectorInfo {
	idSet := make(map[uint64]struct{})
	for _, id := range challengeIDs {
		idSet[id] = struct{}{}
	}

	var filteredSectorInfos []abi.SectorInfo
	for _, si := range allSectorInfos {
		if _, ok := idSet[uint64(si.SectorNumber)]; ok {
			filteredSectorInfos = append(filteredSectorInfos, si)
		}
	}
	return filteredSectorInfos
}
