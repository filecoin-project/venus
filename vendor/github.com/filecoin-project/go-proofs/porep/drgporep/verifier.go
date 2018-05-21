package drgporep

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"

	"github.com/filecoin-project/go-proofs/crypto"
	"github.com/filecoin-project/go-proofs/data"
	"github.com/filecoin-project/go-proofs/porep/drgporep/drgraph"
	"github.com/onrik/gomerkle"
)

// Verifier represents a unique verifier and contains everything needed to verify replication.
type Verifier struct {
	CommD    []byte
	Graph    *drgraph.Graph
	NodeSize int
}

// NewVerifier creates and returns a new Verifier with specified graph and nodeSize.
func NewVerifier(g *drgraph.Graph, nodeSize int) *Verifier {
	return &Verifier{
		Graph:    g,
		NodeSize: nodeSize,
	}
}

// Setup TODO
func (v *Verifier) Setup(data data.ReadWriter) error {
	// 1. Calculate the Merkle root
	tree := v.Graph.MerkleTree(data, v.NodeSize)
	v.CommD = tree.Root()

	return nil
}

// Verify TODO
func (v *Verifier) Verify(challenge int, proverID []byte, commR []byte, proof *Proof) (bool, error) {
	// 1. Validate commitments & merkle proofs
	if ok := v.verifyCommitments(challenge, commR, proof); !ok {
		return false, errors.New("Commitments were incorrect")
	}

	// 2. Use predecessor nodes to run KDF (in order)
	// Sort keys
	var ciphertexts []byte

	keys := make([]int, 0)
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
	data := proof.Node.Data
	unsealed := crypto.Dec(proverID, key, proof.ReplicaNode.Data)
	if !bytes.Equal(data, unsealed) {
		fmt.Println("Summary:")
		fmt.Println("\tchall:\t", challenge)
		fmt.Println("\tparents:", ciphertexts)
		fmt.Println("\tkey:\t", key)
		fmt.Println("\tGOT:\t", unsealed)
		fmt.Println("\tEXPECT:\t", data)
		return false, errors.New("Decrypting seal does not give original data")
	}

	// 4. Check that
	// TODO(@nicola): this depends on the type of graph generation

	return true, nil
}

func (v *Verifier) verifyCommitments(challenge int, commR []byte, proof *Proof) bool {
	tree := gomerkle.NewTree(sha256.New())

	replicaNode := proof.ReplicaNode
	node := proof.Node

	// TODO(@nicola): check hash of the replicaNode.Data matches Hash
	if !tree.VerifyProof(replicaNode.Path, commR, replicaNode.Hash) {
		fmt.Println("replica commitment incorrect")
		fmt.Println(commR)
		fmt.Println(replicaNode.Path)
		fmt.Println(replicaNode.Hash)
		return false
	}

	if !tree.VerifyProof(node.Path, v.CommD, node.Hash) {
		fmt.Println("data commitment incorrect")
		return false
	}

	for i, mProof := range proof.ReplicaParents {
		if !tree.VerifyProof(mProof.Path, commR, mProof.Hash) {
			fmt.Println("replica parent", i, "commitment incorrect")
			return false
		}
	}
	return true
}
