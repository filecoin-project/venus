package proofs

import (
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
	SortedCommRs  SortedCommRs
	Faults        []uint64
	Proofs        []types.PoStProof
	SectorSize    *types.BytesAmount
}

// VerifyPoSTResponse communicates the validity of a provided proof-of-spacetime.
type VerifyPoSTResponse struct {
	IsValid bool
}

// VerifySealResponse communicates the validity of a provided proof-of-replication.
type VerifySealResponse struct {
	IsValid bool
}

// Verifier provides an interface to the proving subsystem.
type Verifier interface {
	VerifyPoST(VerifyPoStRequest) (VerifyPoSTResponse, error)
	VerifySeal(VerifySealRequest) (VerifySealResponse, error)
}
