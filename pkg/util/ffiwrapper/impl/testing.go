package impl

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"

	proof5 "github.com/filecoin-project/specs-actors/v5/actors/runtime/proof"
	proof7 "github.com/filecoin-project/specs-actors/v7/actors/runtime/proof"

	"github.com/filecoin-project/venus/pkg/util/ffiwrapper"
)

// FakeVerifier is a simple mock Verifier for testing.
type FakeVerifier struct {
}

var _ ffiwrapper.Verifier = (*FakeVerifier)(nil)

func (f *FakeVerifier) VerifySeal(proof5.SealVerifyInfo) (bool, error) {
	return true, nil
}

func (f *FakeVerifier) VerifyAggregateSeals(aggregate proof5.AggregateSealVerifyProofAndInfos) (bool, error) {
	return true, nil
}

func (f *FakeVerifier) VerifyReplicaUpdate(update proof7.ReplicaUpdateInfo) (bool, error) {
	return true, nil
}

func (f *FakeVerifier) VerifyWinningPoSt(context.Context, proof5.WinningPoStVerifyInfo) (bool, error) {
	return true, nil
}

func (f *FakeVerifier) VerifyWindowPoSt(context.Context, proof5.WindowPoStVerifyInfo) (bool, error) {
	return true, nil
}

func (f *FakeVerifier) GenerateWinningPoStSectorChallenge(context.Context, abi.RegisteredPoStProof, abi.ActorID, abi.PoStRandomness, uint64) ([]uint64, error) {
	return []uint64{}, nil
}
