package batchedpost

import (
	"github.com/filecoin-project/go-proofs/porep"
)

// Time is a flat circle.
type Time int

// PublicParams are the public parameters for a batched PoSt
type PublicParams struct {
	porep.PublicParams
	t Time
}

// PublicInputs are the public inputs for a batched PoSt
type PublicInputs = porep.PublicInputs

// PrivateInputs are the private inputs for a batched PoSt
type PrivateInputs = porep.PrivateInputs

// Proof is a batched PoSt proof.
type Proof []porep.Proof

// BatchedPost is a proof.ProofScheme.
type BatchedPost struct {
	porep porep.PoRep
}

// New allocates and returns a BatchedPost.
func New(porep porep.PoRep) *BatchedPost {
	return &BatchedPost{porep}
}

// Setup implements proof.ProofScheme.
func (ps *BatchedPost) Setup(setupParams porep.SetupParams) (pp *PublicParams, err error) {
	return pp, err
}

// Prove implements proof.ProofScheme.
func (ps *BatchedPost) Prove(pp *porep.PublicParams, publicInputs *PublicInputs, privateInputs *PrivateInputs) (proof *Proof, err error) {
	return proof, err
}

// Verify implements proof.ProofScheme.
func (ps *BatchedPost) Verify(pp *porep.PublicParams, publicInputs *PublicInputs, proof *porep.Proof) (result bool, err error) {
	return result, err
}
