package drg

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"

	"github.com/filecoin-project/paper-porep-drg/go-porep/drg/crypto"
	"github.com/filecoin-project/paper-porep-drg/go-porep/drg/drgraph"
	"github.com/onrik/gomerkle"
)

type Verifier struct {
	CommE    []byte
	CommR    []byte
	Graph    *drgraph.Graph
	NodeSize int
}

func NewVerifier(g *drgraph.Graph, size int) *Verifier {
	return &Verifier{
		Graph:    g,
		NodeSize: size,
	}
}

func (v *Verifier) Setup(data []byte) {
	// 1. Erasure code the data
	// TODO(@ben): should the verifier erasure code the data as well?
	coded := v.Graph.ErasureCoding(data, v.NodeSize)

	// 2. Calculate the Merkle root
	tree := v.Graph.MerkleTree(coded, v.NodeSize)
	v.CommE = tree.Root()
}

func (v *Verifier) Verify(id []byte, challenge int, proof *Proof) (bool, error) {
	// 1. Validate commitments & merkle proofs
	if ok := v.verifyCommitments(challenge, proof); !ok {
		return false, errors.New("Commitments were incorrect")
	}

	// 2. Use predecessor nodes to run KDF (in order)
	// Sort keys
	keys := make([]int, 0)
	var ciphertexts []byte
	for k := range proof.ReplicaParents {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	// Append them
	for _, k := range keys {
		ciphertexts = append(ciphertexts, proof.ReplicaParents[k].Data...)
	}
	// Generate key
	key := crypto.Kdf(ciphertexts)

	// 3. Validate the correctness of encryption
	// TODO(@nicola): data doesn't need to be in the proof
	data := proof.ErasureNode.Data
	unsealed := crypto.Dec(id, key, proof.ReplicaNode.Data)
	if !bytes.Equal(data, unsealed) {
		fmt.Println("CIP:\t", ciphertexts)
		fmt.Println("KEY:\t", key)
		fmt.Println("GOT:\t", unsealed)
		fmt.Println("EXP:\t", data)
		return false, errors.New("Decrypting seal does not give original data")
	}

	// 4. Check that
	// TODO(@nicola): this depends on the type of graph generation

	return true, nil
}

func (v *Verifier) verifyCommitments(challenge int, proof *Proof) bool {
	tree := gomerkle.NewTree(sha256.New())

	replicaNode := proof.ReplicaNode
	erasureNode := proof.ErasureNode

	// TODO(@nicola): check hash of the replicaNode.Data matches Hash
	if !tree.VerifyProof(replicaNode.Path, v.CommR, replicaNode.Hash) {
		fmt.Println("replica commitment incorrect")
		fmt.Println(v.CommR)
		fmt.Println(replicaNode.Path)
		fmt.Println(replicaNode.Hash)
		return false
	}

	if !tree.VerifyProof(erasureNode.Path, v.CommE, erasureNode.Hash) {
		fmt.Println("erasure commitment incorrect")
		return false
	}

	for i, mProof := range proof.ReplicaParents {
		if !tree.VerifyProof(mProof.Path, v.CommR, mProof.Hash) {
			fmt.Println("replica parent", i, "commitment incorrect")
			return false
		}
	}
	return true
}
