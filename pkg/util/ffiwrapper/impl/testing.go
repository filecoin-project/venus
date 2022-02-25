package impl

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"

	proof7 "github.com/filecoin-project/specs-actors/v7/actors/runtime/proof"

	"github.com/filecoin-project/venus/pkg/util/ffiwrapper"
)

// FakeVerifier is a simple mock Verifier for testing.
type FakeVerifier struct {
}

var _ ffiwrapper.Verifier = (*FakeVerifier)(nil)

func (f *FakeVerifier) VerifySeal(proof7.SealVerifyInfo) (bool, error) {
	return true, nil
}

func (f *FakeVerifier) VerifyAggregateSeals(aggregate proof7.AggregateSealVerifyProofAndInfos) (bool, error) {
	return true, nil
}

func (f *FakeVerifier) VerifyReplicaUpdate(update proof7.ReplicaUpdateInfo) (bool, error) {
	return true, nil
}

func (f *FakeVerifier) VerifyWinningPoSt(ctx context.Context, info proof7.WinningPoStVerifyInfo) (bool, error) {
	return true, nil
}

func (f *FakeVerifier) VerifyWindowPoSt(context.Context, proof7.WindowPoStVerifyInfo) (bool, error) {
	return true, nil
}

func (f *FakeVerifier) GenerateWinningPoStSectorChallenge(ctx context.Context, proofType abi.RegisteredPoStProof, minerID abi.ActorID, randomness abi.PoStRandomness, eligibleSectorCount uint64) ([]uint64, error) {
	return []uint64{}, nil
}
