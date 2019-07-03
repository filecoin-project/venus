package verification

// FakeVerifierConfig configures the behavior of the FakeVerifier.
type FakeVerifierConfig struct {
	VerifyPoStValid                bool
	VerifyPoStError                error
	VerifyPieceInclusionProofValid bool
	VerifyPieceInclusionProofError error
	VerifySealValid                bool
	VerifySealError                error
}

// FakeVerifier is a simple mock Verifier for testing.
type FakeVerifier struct {
	verifyPoStValid                bool
	verifyPoStError                error
	verifyPieceInclusionProofValid bool
	verifyPieceInclusionProofError error
	verifySealValid                bool
	verifySealError                error

	// these requests will be captured by code that calls VerifySeal or VerifyPoSt or VerifyPieceInclusionProof
	LastReceivedVerifySealRequest                *VerifySealRequest
	LastReceivedVerifyPoStRequest                *VerifyPoStRequest
	LastReceivedVerifyPieceInclusionProofRequest *VerifyPieceInclusionProofRequest
}

var _ Verifier = (*FakeVerifier)(nil)

// NewFakeVerifier creates a new FakeVerifier struct.
func NewFakeVerifier(cfg FakeVerifierConfig) *FakeVerifier {
	return &FakeVerifier{
		verifyPoStValid:                cfg.VerifyPoStValid,
		verifyPoStError:                cfg.VerifyPoStError,
		verifyPieceInclusionProofValid: cfg.VerifyPieceInclusionProofValid,
		verifyPieceInclusionProofError: cfg.VerifyPieceInclusionProofError,
		verifySealValid:                cfg.VerifySealValid,
		verifySealError:                cfg.VerifySealError,
	}
}

// VerifyPoSt fakes out PoSt proof verification, skipping an FFI call to
// generate_post.
func (fp *FakeVerifier) VerifyPoSt(req VerifyPoStRequest) (VerifyPoStResponse, error) {
	fp.LastReceivedVerifyPoStRequest = &req
	return VerifyPoStResponse{IsValid: fp.verifyPoStValid}, fp.verifyPoStError
}

// VerifySeal fakes out seal (PoRep) proof verification, skipping an FFI call to
// verify_seal.
func (fp *FakeVerifier) VerifySeal(req VerifySealRequest) (VerifySealResponse, error) {
	fp.LastReceivedVerifySealRequest = &req
	return VerifySealResponse{IsValid: fp.verifySealValid}, fp.verifySealError
}

// VerifyPieceInclusionProof fakes out PIP verification, skipping an FFI call to
// verify_piece_inclusion_proof.
func (fp *FakeVerifier) VerifyPieceInclusionProof(req VerifyPieceInclusionProofRequest) (VerifyPieceInclusionProofResponse, error) {
	fp.LastReceivedVerifyPieceInclusionProofRequest = &req
	return VerifyPieceInclusionProofResponse{IsValid: fp.verifyPieceInclusionProofValid}, fp.verifyPieceInclusionProofError
}
