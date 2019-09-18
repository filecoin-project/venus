package storage_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/protocol/storage"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-sectorbuilder"
)

func TestProver(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	makeAddress := address.NewForTestGetter()
	actorAddress := makeAddress()
	workerAddress := makeAddress()
	sectorSize := types.OneKiBSectorSize

	fakeSeed := types.PoStChallengeSeed{1, 2, 3, 4}
	var fakeInputs []storage.PoStInputs
	start := types.NewBlockHeight(100)
	end := types.NewBlockHeight(200)
	deadline := end.Add(miner.LatePoStGracePeriod(sectorSize))
	collateralRequirement := types.NewAttoFILFromFIL(1)

	makeProofContext := func() *fakeProverContext {
		return &fakeProverContext{
			height:        end.Sub(types.NewBlockHeight(50)), // well inside the window
			actorAddress:  actorAddress,
			workerAddress: workerAddress,
			lateFee:       types.ZeroAttoFIL,
			balance:       collateralRequirement,
			proof:         []byte{1, 2, 3, 4},
			faults:        []uint64{},
		}
	}

	t.Run("produces on-time proof", func(t *testing.T) {
		pc := makeProofContext()
		prover := storage.NewProver(actorAddress, sectorSize, pc, pc)
		submission, e := prover.CalculatePoSt(ctx, start, end, fakeSeed, fakeInputs)
		require.NoError(t, e)
		assert.Equal(t, pc.proof, submission.Proof)
		assert.Equal(t, types.ZeroAttoFIL, submission.Fee)
	})

	t.Run("attaches a fee", func(t *testing.T) {
		pc := makeProofContext()
		pc.lateFee = types.NewAttoFILFromFIL(1)
		// The following block heights should all be capable of attaching a fee if the actor
		// code indicates it should.
		heights := []*types.BlockHeight{
			end.Sub(types.NewBlockHeight(5)),      // A few rounds before on-time window ends
			deadline.Sub(types.NewBlockHeight(5)), // well before deadline
			deadline.Sub(types.NewBlockHeight(1)), // just before deadline
		}

		for _, height := range heights {
			pc.height = height
			prover := storage.NewProver(actorAddress, sectorSize, pc, pc)
			submission, e := prover.CalculatePoSt(ctx, start, end, fakeSeed, fakeInputs)
			require.NoError(t, e)
			assert.Equal(t, pc.proof, submission.Proof)
			assert.True(t, submission.Fee.GreaterThan(types.ZeroAttoFIL))
		}
	})

	t.Run("abandons proof after deadline", func(t *testing.T) {
		pc := makeProofContext()
		pc.height = deadline // proof could only appear in block deadline+1

		prover := storage.NewProver(actorAddress, sectorSize, pc, pc)
		_, e := prover.CalculatePoSt(ctx, start, end, fakeSeed, fakeInputs)
		require.Error(t, e)
	})

	t.Run("abandons proof when chain moves backwards", func(t *testing.T) {
		pc := makeProofContext()
		pc.height = start.Sub(types.NewBlockHeight(1))

		prover := storage.NewProver(actorAddress, sectorSize, pc, pc)
		_, e := prover.CalculatePoSt(ctx, start, end, fakeSeed, fakeInputs)
		require.Error(t, e)
	})

	t.Run("fails without chain height", func(t *testing.T) {
		pc := makeProofContext()
		pc.height = nil

		prover := storage.NewProver(actorAddress, sectorSize, pc, pc)
		_, e := prover.CalculatePoSt(ctx, start, end, fakeSeed, fakeInputs)
		require.Error(t, e)
	})
}

type fakeProverContext struct {
	height        *types.BlockHeight
	actorAddress  address.Address
	workerAddress address.Address
	lateFee       types.AttoFIL
	balance       types.AttoFIL
	proof         types.PoStProof
	faults        []uint64
}

func (f *fakeProverContext) ChainHeadKey() types.TipSetKey {
	return types.NewTipSetKey()
}

func (f *fakeProverContext) ChainTipSet(_ types.TipSetKey) (types.TipSet, error) {
	if f.height == nil {
		return types.TipSet{}, errors.New("could not get the tipset at this height")
	}
	return types.NewTipSet(&types.Block{Height: types.Uint64(f.height.AsBigInt().Uint64())})
}

func (f *fakeProverContext) MinerCalculateLateFee(ctx context.Context, addr address.Address, height *types.BlockHeight) (types.AttoFIL, error) {
	return f.lateFee, nil
}

func (f *fakeProverContext) WalletBalance(ctx context.Context, addr address.Address) (types.AttoFIL, error) {
	if addr == f.workerAddress && !f.balance.IsZero() {
		return f.balance, nil
	}
	return types.ZeroAttoFIL, errors.New("no balance for worker")
}

func (f *fakeProverContext) CalculatePoSt(ctx context.Context, sortedCommRs go_sectorbuilder.SortedSectorInfo, seed types.PoStChallengeSeed) (types.PoStProof, error) {
	return f.proof, nil
}

func (f *fakeProverContext) MinerGetWorkerAddress(_ context.Context, _ address.Address, _ types.TipSetKey) (address.Address, error) {
	return f.workerAddress, nil
}
