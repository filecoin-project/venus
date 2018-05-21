package proofs

// SetupParams implements proofs.ProofScheme
type SetupParams interface{}

// PublicParams implements proofs.ProofScheme
type PublicParams interface{}

// PublicInputs implements proofs.ProofScheme
type PublicInputs interface{}

// PrivateInputs implements proofs.ProofScheme
type PrivateInputs interface{}

// Proof is what Prove returns.
type Proof interface{}

// ProofScheme comprises the methods common to all crypto proof schemes.
type ProofScheme interface {
	// TODO: setup might not be required by all proofs
	Setup(sp SetupParams) (PublicParams, error)
	Prove(pp PublicParams, pubIn PublicInputs, privIn PrivateInputs) (Proof, error)
	Verify(pp PublicParams, pubIn PublicInputs, proof Proof) (bool, error)
}
