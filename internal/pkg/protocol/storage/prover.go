package storage

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-sectorbuilder"
	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

var logProver = logging.Logger("/fil/storage/prover")

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
	// ChainHeadKey returns the cids of the head tipset
	ChainHeadKey() block.TipSetKey
	// ChainTipSet returns the tipset with the given key
	ChainTipSet(key block.TipSetKey) (block.TipSet, error)
	// ChainSampleRandomness returns bytes derived from the blockchain before `sampleHeight`.
	ChainSampleRandomness(ctx context.Context, sampleHeight *types.BlockHeight) ([]byte, error)
	// MinerCalculateLateFee calculates the fee due for a proof submitted at some height.
	MinerCalculateLateFee(ctx context.Context, addr address.Address, height *types.BlockHeight) (types.AttoFIL, error)
	// WalletBalance returns the balance for an actor.
	WalletBalance(ctx context.Context, addr address.Address) (types.AttoFIL, error)
	// MinerGetWorkerAddress returns the current worker address for a miner
	MinerGetWorkerAddress(ctx context.Context, minerAddr address.Address, baseKey block.TipSetKey) (address.Address, error)
}

// ProofCalculator creates the proof-of-spacetime bytes.
type ProofCalculator interface {
	// CalculatePoSt computes a proof-of-spacetime for a list of sector ids and matching seeds.
	// It returns the Snark Proof for the PoSt and a list of sector ids that failed.
	CalculatePoSt(ctx context.Context, sectorInfo go_sectorbuilder.SortedSectorInfo, seed types.PoStChallengeSeed) (types.PoStProof, error)
}

// Prover orchestrates the calculation and submission of a proof-of-spacetime.
type Prover struct {
	actorAddress address.Address
	sectorSize   *types.BytesAmount
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
	Proof    types.PoStProof
	Fee      types.AttoFIL
	GasLimit types.GasUnits
	Faults   types.FaultSet
}

// NewProver constructs a new Prover.
func NewProver(actor address.Address, sectorSize *types.BytesAmount, reader ProofReader, calculator ProofCalculator) *Prover {
	return &Prover{
		actorAddress: actor,
		sectorSize:   sectorSize,
		chain:        reader,
		calculator:   calculator,
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
	sectorInfos := make([]go_sectorbuilder.SectorInfo, len(inputs))
	for i, input := range inputs {
		info := go_sectorbuilder.SectorInfo{
			CommR: input.CommR,
		}
		sectorInfos[i] = info
	}
	logProver.Infof("Prover calculating post for addr %s -- start: %s -- end: %s -- seed: %x", p.actorAddress, start, end, seed)
	for i, ssi := range sectorInfos {
		logProver.Infof("ssi %d: sector id %d -- commR %x", i, ssi.SectorID, ssi.CommR)
	}

	proof, err := p.calculator.CalculatePoSt(ctx, go_sectorbuilder.NewSortedSectorInfo(sectorInfos...), seed)
	if err != nil {
		return nil, errors.Wrap(err, "failed to calculate PoSt")
	}

	// Compute fees.
	headKey := p.chain.ChainHeadKey()
	workerAddr, err := p.chain.MinerGetWorkerAddress(ctx, p.actorAddress, headKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read miner worker address for miner %s", p.actorAddress)
	}

	balance, err := p.chain.WalletBalance(ctx, workerAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to check wallet balance for %s", workerAddr)
	}

	head, err := p.chain.ChainTipSet(headKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve head: %s", headKey.String())
	}
	h, err := head.Height()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get height from head: %s", headKey.String())
	}
	height := types.NewBlockHeight(h)
	if height.LessThan(start) {
		return nil, errors.Errorf("chain height %s is before proving period start %s, abandoning proof", height, start)
	}

	feeDue, err := p.calculateFee(ctx, height, end)
	if err != nil {
		return nil, err
	}
	if feeDue.GreaterThan(balance) {
		log.Warnf("PoSt fee of %s exceeds available balance of %s for owner %s", feeDue, balance, workerAddr)
		// Submit anyway, in case the balance is topped up before the PoSt message is mined.
	}

	return &PoStSubmission{
		Proof:    proof,
		Fee:      feeDue,
		GasLimit: types.NewGasUnits(submitPostGasLimit),
		Faults:   types.EmptyFaultSet(),
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
	gracePeriod := miner.LatePoStGracePeriod(p.sectorSize)
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
		return types.ZeroAttoFIL, errors.Wrapf(err, "Failed to calculate late fee for %s", p.actorAddress)
	}

	return fee, nil
}
