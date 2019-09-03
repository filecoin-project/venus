package storage

import (
	"context"

	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/types"
)

const (
	// Maximum number of rounds delay to allow for when submitting a PoSt for computing any
	// fee necessary due to late submission. The miner expects the PoSt message to be mined
	// into a block at most `buffer` rounds in the future.
	postSubmissionDelayBufferRounds = 10

	// This will likely depend on the sector size and proving period.
	submitPostGasLimit = 300
)

// ProofReader provides information about the blockchain to the proving process.
type ProofReader interface {
	// ChainBlockHeight returns the current height of the best chain.
	ChainBlockHeight() (*types.BlockHeight, error)
	// ChainSampleRandomness returns bytes derived from the blockchain before `sampleHeight`.
	ChainSampleRandomness(ctx context.Context, sampleHeight *types.BlockHeight) ([]byte, error)
	// MinerCalculateLateFee calculates the fee due for a proof submitted at some height.
	MinerCalculateLateFee(ctx context.Context, addr address.Address, height *types.BlockHeight) (types.AttoFIL, error)
	// WalletBalance returns the balance for an actor.
	WalletBalance(ctx context.Context, addr address.Address) (types.AttoFIL, error)
}

// ProofCalculator creates the proof-of-spacetime bytes.
type ProofCalculator interface {
	// CalculatePoSt computes a proof-of-spacetime for a list of sector ids and matching seeds.
	// It returns the Snark Proof for the PoSt and a list of sector ids that failed.
	CalculatePoSt(ctx context.Context, sortedCommRs proofs.SortedCommRs, seed types.PoStChallengeSeed) ([]types.PoStProof, []uint64, error)
}

// Prover orchestrates the calculation and submission of a proof-of-spacetime.
type Prover struct {
	actorAddress  address.Address
	workerAddress address.Address
	sectorSize    *types.BytesAmount
	chain         ProofReader
	calculator    ProofCalculator
}

// PoStInputs contains the sector id and related commitments used to generate a proof-of-spacetime.
type PoStInputs struct {
	CommD     types.CommD
	CommR     types.CommR
	CommRStar types.CommRStar
	SectorID  uint64
}

// PoStSubmission is the information to be submitted on-chain for a proof.
type PoStSubmission struct {
	Proofs   []types.PoStProof
	Fee      types.AttoFIL
	GasLimit types.GasUnits
	Faults   types.FaultSet
}

// NewProver constructs a new Prover.
func NewProver(actor address.Address, worker address.Address, sectorSize *types.BytesAmount, reader ProofReader, calculator ProofCalculator) *Prover {
	return &Prover{
		actorAddress:  actor,
		workerAddress: worker,
		sectorSize:    sectorSize,
		chain:         reader,
		calculator:    calculator,
	}
}

// CalculatePoSt computes and returns a proof-of-spacetime ready for posting on chain.
func (p *Prover) CalculatePoSt(ctx context.Context, start, end *types.BlockHeight, inputs []PoStInputs) (*PoStSubmission, error) {
	// Gather PoSt request inputs.
	seed, err := p.challengeSeed(ctx, start)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch PoSt challenge seed")
	}

	// Compute the actual proof.
	commRs := make([]types.CommR, len(inputs))
	for i, input := range inputs {
		commRs[i] = input.CommR
	}
	proof, faults, err := p.calculator.CalculatePoSt(ctx, proofs.NewSortedCommRs(commRs...), seed)
	if err != nil {
		return nil, errors.Wrap(err, "failed to calculate PoSt")
	}

	if len(faults) != 0 {
		log.Warningf("some faults when generating PoSt: %v", faults)
	}

	// Compute fees.
	balance, err := p.chain.WalletBalance(ctx, p.workerAddress)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to check wallet balance for %s", p.workerAddress)
	}

	height, err := p.chain.ChainBlockHeight()
	if err != nil {
		return nil, errors.Wrap(err, "failed to check chain height")
	}
	if height.LessThan(start) {
		return nil, errors.Errorf("chain height %s is before proving period start %s, abandoning proof", height, start)
	}

	feeDue, err := p.calculateFee(ctx, height, end)
	if err != nil {
		return nil, err
	}
	if feeDue.GreaterThan(balance) {
		log.Warningf("PoSt fee of %s exceeds available balance of %s for owner %s", feeDue, balance, p.workerAddress)
		// Submit anyway, in case the balance is topped up before the PoSt message is mined.
	}

	return &PoStSubmission{
		Proofs:   proof,
		Fee:      feeDue,
		GasLimit: types.NewGasUnits(submitPostGasLimit),
		Faults:   types.NewFaultSet(faults),
	}, nil
}

// challengeSeed returns the PoSt challenge seed for a proving period.
func (p *Prover) challengeSeed(ctx context.Context, periodStart *types.BlockHeight) (types.PoStChallengeSeed, error) {
	bytes, err := p.chain.ChainSampleRandomness(ctx, periodStart)
	if err != nil {
		return types.PoStChallengeSeed{}, err
	}

	seed := types.PoStChallengeSeed{}
	copy(seed[:], bytes)
	return seed, nil
}

// calculateFee calculates any fees due with a proof submission due to faults or lateness.
func (p *Prover) calculateFee(ctx context.Context, height *types.BlockHeight, end *types.BlockHeight) (types.AttoFIL, error) {
	gracePeriod := miner.GenerationAttackTime(p.sectorSize)
	deadline := end.Add(gracePeriod)
	if height.GreaterEqual(deadline) {
		// The generation attack time has expired and the proof will be rejected.
		// The earliest the proof could be mined is round (deadline+1).
		// The miner can expect to be slashed of all its collateral and power.
		return types.ZeroAttoFIL, errors.Errorf("PoSt generation was too slow height=%s end=%s deadline=%s", height, end, deadline)
	}

	expectedProofHeight := height.Add(types.NewBlockHeight(postSubmissionDelayBufferRounds))
	fee, err := p.chain.MinerCalculateLateFee(ctx, p.actorAddress, expectedProofHeight)
	if err != nil {
		return types.ZeroAttoFIL, errors.Errorf("Failed to calculate late fee for %s", p.actorAddress)
	}

	return fee, nil
}
