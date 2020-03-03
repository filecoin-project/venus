package submodule

import (
	"github.com/filecoin-project/go-sectorbuilder"
)

// ProofVerificationSubmodule adds proof verification capabilities to the node.
type ProofVerificationSubmodule struct {
	ProofVerifier sectorbuilder.Verifier
}

// NewProofVerificationSubmodule creates a new proof verification submodule.
func NewProofVerificationSubmodule(verifier sectorbuilder.Verifier) ProofVerificationSubmodule {
	return ProofVerificationSubmodule{
		ProofVerifier: verifier,
	}
}
