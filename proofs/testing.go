package proofs

// FakeProver is a simple mock Verifier for testing
type FakeProver struct {
	verifyPostValid bool
	verifyPostError error
}

// NewFakeProver creates a new FakeProver struct
func NewFakeProver(isValid bool, err error) FakeProver {
	return FakeProver{isValid, err}
}

// GeneratePoST panics. It fulfils a requirement for the Verifier interface
func (FakeProver) GeneratePoST(GeneratePoSTRequest) (GeneratePoSTResponse, error) {
	panic("boom")
}

// VerifyPoST returns the valid of verifyPostValid and verifyPostError.
// It fulfils a requirement for the Verifier interface
func (fp FakeProver) VerifyPoST(VerifyPoSTRequest) (VerifyPoSTResponse, error) {
	return VerifyPoSTResponse{IsValid: fp.verifyPostValid}, fp.verifyPostError
}

// VerifySeal panics. It fulfils a requirement for the Verifier interface
func (FakeProver) VerifySeal(VerifySealRequest) (VerifySealResponse, error) {
	panic("boom")
}
