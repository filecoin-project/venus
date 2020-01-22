package submodule

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs/verification"
)

// ProofVerificationSubmodule adds proof verification capabilities to the node.
type ProofVerificationSubmodule struct {
	ProofVerifier verification.Verifier
}

// NewProofVerificationSubmodule creates a new proof verification submodule.
func NewProofVerificationSubmodule() ProofVerificationSubmodule {
	return ProofVerificationSubmodule{
		ProofVerifier: verification.NewFFIBackedProofVerifier(),
	}
}
