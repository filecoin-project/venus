package proofs

import (
	"context"

	"github.com/filecoin-project/sector-storage/ffiwrapper"
	"github.com/filecoin-project/specs-actors/actors/abi"
)

// FakeVerifier is a simple mock Verifier for testing.
type FakeVerifier struct {
}

var _ ffiwrapper.Verifier = (*FakeVerifier)(nil)

func (f *FakeVerifier) VerifySeal(abi.SealVerifyInfo) (bool, error) {
	return true, nil
}

func (f *FakeVerifier) VerifyWinningPoSt(context.Context, abi.WinningPoStVerifyInfo) (bool, error) {
	return true, nil
}

func (f *FakeVerifier) VerifyWindowPoSt(context.Context, abi.WindowPoStVerifyInfo) (bool, error) {
	return true, nil
}

func (f *FakeVerifier) GenerateWinningPoStSectorChallenge(context.Context, abi.RegisteredProof, abi.ActorID, abi.PoStRandomness, uint64) ([]uint64, error) {
	return []uint64{}, nil
}
