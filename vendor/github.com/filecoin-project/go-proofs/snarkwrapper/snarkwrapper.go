package snarkwrapper

import (
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-proofs/proofs"
	"github.com/filecoin-project/go-proofs/snark"
)

// Circuit is a SNARK circuit.
// Should this live in the snark package?
type Circuit struct{}

// SNARKWrapper wraps a proofScheme and a SNARK circuit and implements proofs.ProofScheme.
type SNARKWrapper struct {
	proofScheme proofs.ProofScheme
	circuit     Circuit
}

// Setup implements proofs.ProofScheme.
func (sw *SNARKWrapper) Setup(sp proofs.SetupParams) (proofs.PublicParams, error) {
	return sw.proofScheme.Setup(sp)
}

// Prove implements proofs.ProofScheme.
func (sw *SNARKWrapper) Prove(pp proofs.PublicParams, pubIn proofs.PublicInputs, privIn proofs.PrivateInputs) (proofs.Proof, error) {
	proof, err := sw.proofScheme.Prove(pp, pubIn, privIn)
	if err != nil {
		return nil, errors.Wrap(err, "inner proof failed")
	}

	return snark.S.Prove(sw.circuit, pubIn, proof)
}

// Verify implements proofs.ProofScheme.
func (sw *SNARKWrapper) Verify(pp proofs.PublicParams, pubIn proofs.PublicInputs, proof proofs.Proof) (bool, error) {
	return snark.S.Verify(sw.circuit, pubIn, proof)
}
