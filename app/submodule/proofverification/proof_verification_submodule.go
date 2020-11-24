package proofverification

import (
	"github.com/filecoin-project/venus/pkg/util/ffiwrapper"
)

// ProofVerificationSubmodule adds proof verification capabilities to the node.
type ProofVerificationSubmodule struct { //nolint
	ProofVerifier ffiwrapper.Verifier
}

// NewProofVerificationSubmodule creates a new proof verification submodule.
func NewProofVerificationSubmodule(verifier ffiwrapper.Verifier) *ProofVerificationSubmodule {
	return &ProofVerificationSubmodule{
		ProofVerifier: verifier,
	}
}

func (proofVerification *ProofVerificationSubmodule) API() *ProofVerificationApi {
	return &ProofVerificationApi{proofVerification: proofVerification}
}
