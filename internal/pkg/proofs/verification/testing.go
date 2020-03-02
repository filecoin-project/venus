package verification

import (
	"github.com/filecoin-project/specs-actors/actors/abi"
)

// VerifySealRequest represents a request to verify the output of a Seal() operation.
type VerifySealRequest struct {
	input abi.SealVerifyInfo
}

// VerifyPoStRequest represents a request to generate verify a proof-of-spacetime.
type VerifyPoStRequest struct {
	input abi.PoStVerifyInfo
}

// FakeVerifier is a simple mock Verifier for testing.
type FakeVerifier struct {
	VerifyPoStValid bool
	VerifyPoStError error
	VerifySealValid bool
	VerifySealError error

	// these requests will be captured by code that calls VerifySeal or VerifyPoSt
	LastReceivedVerifySealRequest *VerifySealRequest
	LastReceivedVerifyPoStRequest *VerifyPoStRequest
}

var _ PoStVerifier = (*FakeVerifier)(nil)
var _ SealVerifier = (*FakeVerifier)(nil)

// VerifySeal fakes out seal (PoRep) proof verification, skipping an FFI call to
// verify_seal.
func (f *FakeVerifier) VerifySeal(info abi.SealVerifyInfo) (bool, error) {
	f.LastReceivedVerifySealRequest = &VerifySealRequest{
		input: info,
	}
	return f.VerifySealValid, f.VerifySealError
}

// VerifyPoSt fakes out PoSt proof verification, skipping an FFI call to
// generate_post.
func (f *FakeVerifier) VerifyPoSt(info abi.PoStVerifyInfo) (bool, error) {
	f.LastReceivedVerifyPoStRequest = &VerifyPoStRequest{
		input: info,
	}
	return f.VerifyPoStValid, f.VerifyPoStError
}
