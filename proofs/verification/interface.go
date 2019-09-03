package verification

import (
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/types"
)

// VerifySealRequest represents a request to verify the output of a Seal() operation.
type VerifySealRequest struct {
	CommD      types.CommD      // returned from seal
	CommR      types.CommR      // returned from seal
	CommRStar  types.CommRStar  // returned from seal
	Proof      types.PoRepProof // returned from seal
	ProverID   [31]byte         // uniquely identifies miner
	SectorID   [31]byte         // uniquely identifies sector
	SectorSize *types.BytesAmount
}

// VerifyPoStRequest represents a request to generate verify a proof-of-spacetime.
type VerifyPoStRequest struct {
	ChallengeSeed types.PoStChallengeSeed
	SortedCommRs  proofs.SortedCommRs
	Faults        []uint64
	Proofs        []types.PoStProof
	SectorSize    *types.BytesAmount
}

// VerifyPoStResponse communicates the validity of a provided proof-of-spacetime.
type VerifyPoStResponse struct {
	IsValid bool
}

// VerifySealResponse communicates the validity of a provided proof-of-replication.
type VerifySealResponse struct {
	IsValid bool
}

// VerifyPieceInclusionProofRequest represents a request to verify a piece
// inclusion proof.
type VerifyPieceInclusionProofRequest struct {
	CommD               types.CommD
	CommP               types.CommP
	PieceInclusionProof []byte
	PieceSize           *types.BytesAmount
	SectorSize          *types.BytesAmount
}

// VerifyPieceInclusionProofResponse communicates the validity of a provided
// piece inclusion proof.
type VerifyPieceInclusionProofResponse struct {
	IsValid bool
}

// Verifier provides an interface to the proving subsystem.
type Verifier interface {
	VerifyPoSt(VerifyPoStRequest) (VerifyPoStResponse, error)
	VerifySeal(VerifySealRequest) (VerifySealResponse, error)
	VerifyPieceInclusionProof(VerifyPieceInclusionProofRequest) (VerifyPieceInclusionProofResponse, error)
}
