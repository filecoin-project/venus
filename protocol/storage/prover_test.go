package storage_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	ownerAddress := makeAddress()

	var fakeSeed types.PoStChallengeSeed
	fakeSeed[0] = 1
	var fakeInputs []storage.PoStInputs
	start := types.NewBlockHeight(100)
	end := types.NewBlockHeight(200)

	t.Run("produces proof", func(t *testing.T) {
		fake := &fakeProverDeps{
			seed:   fakeSeed,
			height: end.Sub(types.NewBlockHeight(1)),
			proofs: []types.PoStProof{{1, 2, 3, 4}},
			faults: []uint64{},
		}
		prover := storage.NewProver(actorAddress, ownerAddress, fake, fake)

		submission, e := prover.CalculatePoSt(ctx, start, end, fakeInputs)
		require.NoError(t, e)
		assert.Equal(t, fake.proofs, submission.Proofs)
	})

	t.Run("fails without chain height", func(t *testing.T) {
		fake := &fakeProverDeps{
			seed:   fakeSeed,
			height: nil,
			proofs: []types.PoStProof{{1, 2, 3, 4}},
			faults: []uint64{},
		}
		prover := storage.NewProver(actorAddress, ownerAddress, fake, fake)

		_, e := prover.CalculatePoSt(ctx, start, end, fakeInputs)
		require.Error(t, e)
	})

	t.Run("fails without challenge seed", func(t *testing.T) {
		fake := &fakeProverDeps{
			seed:   zeroSeed,
			height: end.Sub(types.NewBlockHeight(1)),
			proofs: []types.PoStProof{{1, 2, 3, 4}},
			faults: []uint64{},
		}
		prover := storage.NewProver(actorAddress, ownerAddress, fake, fake)

		_, e := prover.CalculatePoSt(ctx, start, end, fakeInputs)
		require.Error(t, e)
	})
}

type fakeProverDeps struct {
	seed   types.PoStChallengeSeed
	height *types.BlockHeight
	proofs []types.PoStProof
	faults []uint64
}

func (f *fakeProverDeps) ChainHeight() (*types.BlockHeight, error) {
	if f.height != nil {
		return f.height, nil
	}
	return nil, errors.New("no height")
}

func (f *fakeProverDeps) ChallengeSeed(ctx context.Context, periodStart *types.BlockHeight) (types.PoStChallengeSeed, error) {
	if f.seed != zeroSeed {
		return f.seed, nil
	}
	return zeroSeed, errors.New("no seed")
}

func (f *fakeProverDeps) CalculatePost(sortedCommRs proofs.SortedCommRs, seed types.PoStChallengeSeed) ([]types.PoStProof, []uint64, error) {
	return f.proofs, f.faults, nil
}
