package proofs

// FakeVerifier is a simple mock Verifier for testing
type FakeVerifier struct {
	verifyPostValid bool
	verifyPostError error
}

// NewFakeVerifier creates a new FakeVerifier struct
func NewFakeVerifier(isValid bool, err error) FakeVerifier {
	return FakeVerifier{isValid, err}
}

// VerifyPoST returns the valid of verifyPostValid and verifyPostError.
// It fulfils a requirement for the Verifier interface
func (fp FakeVerifier) VerifyPoST(VerifyPoStRequest) (VerifyPoSTResponse, error) {
	return VerifyPoSTResponse{IsValid: fp.verifyPostValid}, fp.verifyPostError
}

// VerifySeal panics. It fulfils a requirement for the Verifier interface
func (FakeVerifier) VerifySeal(VerifySealRequest) (VerifySealResponse, error) {
	panic("boom")
}
