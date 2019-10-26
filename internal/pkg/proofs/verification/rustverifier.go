package verification

import (
	"github.com/filecoin-project/go-sectorbuilder"
)

// RustVerifier provides proof-verification methods.
type RustVerifier struct{}

var _ Verifier = &RustVerifier{}

// VerifySeal returns nil if the Seal operation from which its inputs were
// derived was valid, and an error if not.
func (rp *RustVerifier) VerifySeal(req VerifySealRequest) (VerifySealResponse, error) {
	isValid, err := go_sectorbuilder.VerifySeal(req.SectorSize.Uint64(), req.CommR, req.CommD, req.CommRStar, req.ProverID, req.SectorID, req.Proof)
	if err != nil {
		return VerifySealResponse{}, err
	}

	return VerifySealResponse{
		IsValid: isValid,
	}, nil
}

// VerifyPoSt verifies that a proof-of-spacetime is valid.
func (rp *RustVerifier) VerifyPoSt(req VerifyPoStRequest) (VerifyPoStResponse, error) {
	isValid, err := go_sectorbuilder.VerifyPoSt(req.SectorSize.Uint64(), req.SortedSectorInfo, req.ChallengeSeed, req.Proof, req.Faults)
	if err != nil {
		return VerifyPoStResponse{}, err
	}

	return VerifyPoStResponse{
		IsValid: isValid,
	}, nil
}

// VerifyPieceInclusionProof returns true if the piece inclusion proof is valid
// with the given arguments.
func (rp *RustVerifier) VerifyPieceInclusionProof(req VerifyPieceInclusionProofRequest) (VerifyPieceInclusionProofResponse, error) {
	isValid, err := go_sectorbuilder.VerifyPieceInclusionProof(req.SectorSize.Uint64(), req.PieceSize.Uint64(), req.CommP, req.CommD, req.PieceInclusionProof)
	if err != nil {
		return VerifyPieceInclusionProofResponse{}, err
	}

	return VerifyPieceInclusionProofResponse{
		IsValid: isValid,
	}, nil
}
