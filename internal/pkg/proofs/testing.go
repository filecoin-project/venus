package proofs

import (
	"context"
	"github.com/filecoin-project/specs-actors/actors/runtime/proof"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/internal/pkg/util/ffiwrapper"
)

// FakeVerifier is a simple mock Verifier for testing.
type FakeVerifier struct {
}

var _ ffiwrapper.Verifier = (*FakeVerifier)(nil)

func (f *FakeVerifier) VerifySeal(proof.SealVerifyInfo) (bool, error) {
	return true, nil
}

func (f *FakeVerifier) VerifyWinningPoSt(context.Context, proof.WinningPoStVerifyInfo) (bool, error) {
	return true, nil
}

func (f *FakeVerifier) VerifyWindowPoSt(context.Context, proof.WindowPoStVerifyInfo) (bool, error) {
	return true, nil
}

func (f *FakeVerifier) GenerateWinningPoStSectorChallenge(context.Context, abi.RegisteredPoStProof, abi.ActorID, abi.PoStRandomness, uint64) ([]uint64, error) {
	return []uint64{}, nil
}
