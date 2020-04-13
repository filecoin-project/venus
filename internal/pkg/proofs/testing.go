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

func (f *FakeVerifier) VerifyElectionPost(context.Context, abi.PoStVerifyInfo) (bool, error) {
	return true, nil
}

func (f *FakeVerifier) VerifyFallbackPost(context.Context, abi.PoStVerifyInfo) (bool, error) {
	return true, nil
}
