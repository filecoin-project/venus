package snark

import "github.com/filecoin-project/go-proofs/proofs"

// SNARK represents a succinct non-interactive argument for knowledge!
type SNARK struct {
	// TODO
}

// PublicParams are the public parameters for a SNARK proof.
type PublicParams struct {
	Snark interface{}
}

// New allocates and returns a SNARK.
func New() *SNARK {
	return &SNARK{}
}

// S is a singleton handle on SNARK methods.
var S = New()

// SimpleProof is the private input to SNARK's Prove method.
// It is the result of a call to a non-SNARK Prove method.
type SimpleProof = proofs.PrivateInputs

// Proof is a SNARK proof.
type Proof struct {
	// TODO
}

// Setup implements proofs.ProofScheme.
func (s *SNARK) Setup(sp proofs.SetupParams) (proofs.PublicParams, error) {
	return nil, nil // FIXME
}

// Prove implements proofs.ProofScheme.
func (s *SNARK) Prove(pp proofs.PublicParams, pubIn proofs.PublicInputs, simpleProof SimpleProof) (proofs.Proof, error) {
	return proofs.Proof(&Proof{}), nil // FIXME
}

// Verify implements proofs.Verify.
func (s *SNARK) Verify(pp proofs.PublicParams, pubIn proofs.PublicInputs, proof proofs.Proof) (bool, error) {
	return false, nil // FIXME
}
