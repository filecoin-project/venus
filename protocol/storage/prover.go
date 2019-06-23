package storage

import (
	"context"

	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/types"
)

// This will likely depend on the sector size and proving period.
const submitPostGasLimit = 300

// ProofReader provides information about the blockchain to the proving process.
type ProofReader interface {
	// ChainHeight returns the current height of the best chain.
	ChainHeight() (*types.BlockHeight, error)
	// ChallengeSeed returns the PoSt challenge seed for a proving period.
	ChallengeSeed(ctx context.Context, periodStart *types.BlockHeight) (types.PoStChallengeSeed, error)
}

// ProofCalculator creates the proof-of-spacetime bytes.
type ProofCalculator interface {
	// CalculatePost computes a proof-of-spacetime for a list of sector ids and matching seeds.
	// It returns the Snark Proof for the PoSt and a list of sector ids that failed.
	CalculatePost(sortedCommRs proofs.SortedCommRs, seed types.PoStChallengeSeed) ([]types.PoStProof, []uint64, error)
}

// Prover orchestrates the calculation and submission of a proof-of-spacetime.
type Prover struct {
	actorAddress address.Address
	ownerAddress address.Address
	chain        ProofReader
	calculator   ProofCalculator
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
}

// NewProver constructs a new Prover.
func NewProver(actor address.Address, owner address.Address, reader ProofReader, caculator ProofCalculator) *Prover {
	return &Prover{
		actorAddress: actor,
		ownerAddress: owner,
		chain:        reader,
		calculator:   caculator,
	}
}

// CalculatePoSt computes and returns a proof-of-spacetime ready for posting on chain.
func (sm *Prover) CalculatePoSt(ctx context.Context, start, end *types.BlockHeight, inputs []PoStInputs) (*PoStSubmission, error) {
	// Gather PoSt request inputs.
	seed, err := sm.chain.ChallengeSeed(ctx, start)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch PoSt challenge seed")
	}

	// Compute the actual proof.
	commRs := make([]types.CommR, len(inputs))
	for i, input := range inputs {
		commRs[i] = input.CommR
	}
	proofs, faults, err := sm.calculator.CalculatePost(proofs.NewSortedCommRs(commRs...), seed)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate PoSt")
	}

	// Compute fees.
	if len(faults) != 0 {
		log.Warningf("some faults when generating PoSt: %v", faults)
		// TODO: include faults in submission https://github.com/filecoin-project/go-filecoin/issues/2889
	}

	height, err := sm.chain.ChainHeight()
	if err != nil {
		// TODO: what should happen in this case?
		return nil, errors.Errorf("failed to submit PoSt, as the current block height can not be determined: %s", err)
	}
	if height.LessThan(start) {
		// TODO: what to do here? not sure this can happen, maybe through reordering?
		return nil, errors.Errorf("PoSt generation time took negative block time: %s < %s", height, start)

	}

	if height.GreaterEqual(end) {
		// TODO: pay late fees https://github.com/filecoin-project/go-filecoin/issues/2942
		return nil, errors.Errorf("PoSt generation was too slow height=%s end=%s", height, end)
	}

	return &PoStSubmission{
		Proofs:   proofs,
		Fee:      types.ZeroAttoFIL,
		GasLimit: types.NewGasUnits(submitPostGasLimit),
	}, nil
}
