package proofs

import "C"

// RustVerifier provides proof-verification methods.
type RustVerifier struct{}

var _ Verifier = &RustVerifier{}

// VerifySeal returns nil if the Seal operation from which its inputs were
// derived was valid, and an error if not.
func (rp *RustVerifier) VerifySeal(req VerifySealRequest) (VerifySealResponse, error) {
	isValid, err := VerifySeal(req.SectorSize, req.CommR, req.CommD, req.CommRStar, req.ProverID, req.SectorID, req.Proof)
	if err != nil {
		return VerifySealResponse{}, err
	}

	return VerifySealResponse{
		IsValid: isValid,
	}, nil
}

// VerifyPoSt verifies that a proof-of-spacetime is valid.
func (rp *RustVerifier) VerifyPoSt(req VerifyPoStRequest) (VerifyPoStResponse, error) {
	isValid, err := VerifyPoSt(req.SectorSize, req.SortedCommRs, req.ChallengeSeed, req.Proofs, req.Faults)
	if err != nil {
		return VerifyPoStResponse{}, err
	}

	return VerifyPoStResponse{
		IsValid: isValid,
	}, nil
}
