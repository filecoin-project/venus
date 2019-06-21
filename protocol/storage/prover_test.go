package storage_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/protocol/storage"
	"github.com/filecoin-project/go-filecoin/types"
)

var zeroSeed types.PoStChallengeSeed

func TestProver(t *testing.T) {
	ctx := context.Background()
	makeAddress := address.NewForTestGetter()
	actorAddress := makeAddress()
	workerAddress := makeAddress()
	sectorSize := types.OneKiBSectorSize

	var fakeSeed types.PoStChallengeSeed
	fakeSeed[0] = 1
	var fakeInputs []storage.PoStInputs
	start := types.NewBlockHeight(100)
	end := types.NewBlockHeight(200)
	deadline := end.Add(miner.GenerationAttackTime(sectorSize))
	collateralRequirement := types.NewAttoFILFromFIL(1)

	makeProofContext := func() *fakeProverContext {
		return &fakeProverContext{
			height:        end.Sub(types.NewBlockHeight(50)), // well inside the window
			seed:          fakeSeed,
			actorAddress:  actorAddress,
			workerAddress: workerAddress,
			collateral:    collateralRequirement,
			balance:       collateralRequirement,
			proofs:        []types.PoStProof{{1, 2, 3, 4}},
			faults:        []uint64{},
		}
	}

	t.Run("produces on-time proof", func(t *testing.T) {
		pc := makeProofContext()
		prover := storage.NewProver(actorAddress, workerAddress, sectorSize, pc, pc)
		submission, e := prover.CalculatePoSt(ctx, start, end, fakeInputs)
		require.NoError(t, e)
		assert.Equal(t, pc.proofs, submission.Proofs)
		assert.Equal(t, types.ZeroAttoFIL, submission.Fee)
	})

	t.Run("attaches a fee some buffer before lateness", func(t *testing.T) {
		pc := makeProofContext()
		pc.height = end.Sub(types.NewBlockHeight(5)) // a few rounds before on-time window ends
		prover := storage.NewProver(actorAddress, workerAddress, sectorSize, pc, pc)
		submission, e := prover.CalculatePoSt(ctx, start, end, fakeInputs)
		require.NoError(t, e)
		assert.Equal(t, pc.proofs, submission.Proofs)
		// A fee is attached to allow for the proof being mined in the future, when it might be late.
		assert.True(t, submission.Fee.GreaterThan(types.ZeroAttoFIL))
	})

	t.Run("attaches max fee at deadline", func(t *testing.T) {
		pc := makeProofContext()
		pc.height = deadline.Sub(types.NewBlockHeight(1)) // just before deadline
		prover := storage.NewProver(actorAddress, workerAddress, sectorSize, pc, pc)
		submission, e := prover.CalculatePoSt(ctx, start, end, fakeInputs)
		require.NoError(t, e)
		assert.Equal(t, pc.proofs, submission.Proofs)
		assert.Equal(t, collateralRequirement, submission.Fee)
	})

	t.Run("attaches max fee at some buffer before deadline", func(t *testing.T) {
		pc := makeProofContext()
		pc.height = deadline.Sub(types.NewBlockHeight(5)) // well before deadline
		prover := storage.NewProver(actorAddress, workerAddress, sectorSize, pc, pc)
		submission, e := prover.CalculatePoSt(ctx, start, end, fakeInputs)
		require.NoError(t, e)
		assert.Equal(t, pc.proofs, submission.Proofs)
		assert.Equal(t, collateralRequirement, submission.Fee)
	})

	t.Run("abandons proof after deadline", func(t *testing.T) {
		pc := makeProofContext()
		pc.height = deadline // proof could only appear in block deadline+1

		prover := storage.NewProver(actorAddress, workerAddress, sectorSize, pc, pc)
		_, e := prover.CalculatePoSt(ctx, start, end, fakeInputs)
		require.Error(t, e)
	})

	t.Run("abandons proof when chain moves backwards", func(t *testing.T) {
		pc := makeProofContext()
		pc.height = start.Sub(types.NewBlockHeight(1))

		prover := storage.NewProver(actorAddress, workerAddress, sectorSize, pc, pc)
		_, e := prover.CalculatePoSt(ctx, start, end, fakeInputs)
		require.Error(t, e)
	})

	t.Run("fails without chain height", func(t *testing.T) {
		pc := makeProofContext()
		pc.height = nil

		prover := storage.NewProver(actorAddress, workerAddress, sectorSize, pc, pc)
		_, e := prover.CalculatePoSt(ctx, start, end, fakeInputs)
		require.Error(t, e)
	})

	t.Run("fails without challenge seed", func(t *testing.T) {
		pc := makeProofContext()
		pc.seed = zeroSeed

		prover := storage.NewProver(actorAddress, workerAddress, sectorSize, pc, pc)
		_, e := prover.CalculatePoSt(ctx, start, end, fakeInputs)
		require.Error(t, e)
	})
}

type fakeProverContext struct {
	height        *types.BlockHeight
	seed          types.PoStChallengeSeed
	actorAddress  address.Address
	workerAddress address.Address
	collateral    types.AttoFIL
	balance       types.AttoFIL
	proofs        []types.PoStProof
	faults        []uint64
}

func (f *fakeProverContext) ChainHeight() (*types.BlockHeight, error) {
	if f.height != nil {
		return f.height, nil
	}
	return nil, errors.New("no height")
}

func (f *fakeProverContext) ChallengeSeed(ctx context.Context, periodStart *types.BlockHeight) (types.PoStChallengeSeed, error) {
	if f.seed != zeroSeed {
		return f.seed, nil
	}
	return zeroSeed, errors.New("no seed")
}

func (f *fakeProverContext) PledgeCollateralRequirement(ctx context.Context, addr address.Address) (types.AttoFIL, error) {
	if addr == f.actorAddress && !f.collateral.IsZero() {
		return f.collateral, nil
	}
	return types.ZeroAttoFIL, errors.New("no collateral for actor")
}

func (f *fakeProverContext) WalletBalance(ctx context.Context, addr address.Address) (types.AttoFIL, error) {
	if addr == f.workerAddress && !f.balance.IsZero() {
		return f.balance, nil
	}
	return types.ZeroAttoFIL, errors.New("no balance for worker")
}

func (f *fakeProverContext) CalculatePost(sortedCommRs proofs.SortedCommRs, seed types.PoStChallengeSeed) ([]types.PoStProof, []uint64, error) {
	return f.proofs, f.faults, nil
}
