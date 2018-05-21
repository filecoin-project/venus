package drgporep

import (
	"github.com/filecoin-project/go-proofs/crypto"
	"github.com/filecoin-project/go-proofs/porep"
)

// DrgPorep implements Proof Of Replication with a Depth-Robust Graph.
type DrgPorep struct{}

// Proof specifies the structure of a drgporep proof.
type Proof struct {
	ReplicaNode    *crypto.MerkleProof
	ReplicaParents map[int]*crypto.MerkleProof
	Node           *crypto.MerkleProof
}

// Tau TODO
type Tau struct {
	CommR porep.Commitment
	CommD porep.Commitment
}

// Setup implements proofs.ProofScheme
func (ps *DrgPorep) Setup(setupParams porep.SetupParams) (pp *porep.PublicParams, err error) {
	return pp, err
}

// Prove implements proofs.ProofScheme
func (ps *DrgPorep) Prove(pp *porep.PublicParams, publicInputs *porep.PublicInputs, privateInputs *porep.PrivateInputs) (proof *Proof, err error) {
	return proof, err
}

// Verify implements proofs.ProofScheme
func (ps *DrgPorep) Verify(pp *porep.PublicParams, publicInputs *porep.PublicInputs, proof *Proof) (result bool, err error) {
	return result, err
}

// Replicate implements porep.PoRep
func (ps *DrgPorep) Replicate(pp *porep.PublicParams, id porep.ProverID, data []byte) (replica *porep.Replica, tau *porep.Tau, err error) {
	return replica, tau, err
}

// ExtractAll implements porep.PoRep
func (ps *DrgPorep) ExtractAll(pp *porep.PublicParams, id porep.ProverID, replica porep.Replica) (data []byte, err error) {
	return data, err
}

// Extract implements porep.PoRep. It extracts a single node by id.
func (ps *DrgPorep) Extract(pp *porep.PublicParams, id porep.ProverID, replica porep.Replica, nodeID int) (data []byte, err error) {
	return data, err
}
