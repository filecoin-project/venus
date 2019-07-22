package verification

import (
	"github.com/filecoin-project/go-filecoin/proofs/libsectorbuilder"
)

// RustVerifier provides proof-verification methods.
type RustVerifier struct{}

var _ Verifier = &RustVerifier{}

// VerifySeal returns nil if the Seal operation from which its inputs were
// derived was valid, and an error if not.
func (rp *RustVerifier) VerifySeal(req VerifySealRequest) (VerifySealResponse, error) {
	isValid, err := libsectorbuilder.VerifySeal(req.SectorSize.Uint64(), req.CommR, req.CommD, req.CommRStar, req.ProverID, req.SectorID, req.Proof)
	if err != nil {
		return VerifySealResponse{}, err
	}

	return VerifySealResponse{
		IsValid: isValid,
	}, nil
}

// VerifyPoSt verifies that a proof-of-spacetime is valid.
func (rp *RustVerifier) VerifyPoSt(req VerifyPoStRequest) (VerifyPoStResponse, error) {
	sortedCommRs := req.SortedCommRs.Values()

	asArrays := make([][32]byte, len(sortedCommRs))
	for idx, comm := range sortedCommRs {
		copy(asArrays[idx][:], comm[:])
	}

	asSlices := make([][]byte, len(req.Proofs))
	for idx, proof := range req.Proofs {
		asSlices[idx] = append(proof[:0:0], proof...)
	}

	isValid, err := libsectorbuilder.VerifyPoSt(req.SectorSize.Uint64(), asArrays, req.ChallengeSeed, asSlices, req.Faults)
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
	isValid, err := libsectorbuilder.VerifyPieceInclusionProof(req.SectorSize.Uint64(), req.PieceSize.Uint64(), req.CommP, req.CommD, req.PieceInclusionProof)
	if err != nil {
		return VerifyPieceInclusionProofResponse{}, err
	}

	return VerifyPieceInclusionProofResponse{
		IsValid: isValid,
	}, nil
}
