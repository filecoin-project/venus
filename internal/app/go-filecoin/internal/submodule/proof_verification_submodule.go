package submodule

import (
	"github.com/filecoin-project/venus/internal/pkg/util/ffiwrapper"
)

// ProofVerificationSubmodule adds proof verification capabilities to the node.
type ProofVerificationSubmodule struct {
	ProofVerifier ffiwrapper.Verifier
}

// NewProofVerificationSubmodule creates a new proof verification submodule.
func NewProofVerificationSubmodule(verifier ffiwrapper.Verifier) ProofVerificationSubmodule {
	return ProofVerificationSubmodule{
		ProofVerifier: verifier,
	}
}
