package drgporep

import (
	"github.com/filecoin-project/go-proofs/crypto"
	"github.com/filecoin-project/go-proofs/data"
	"github.com/filecoin-project/go-proofs/porep/drgporep/drgraph"
	"github.com/onrik/gomerkle"
)

// Prover represents a unique prover and contains everything needed to prove replication.
type Prover struct {
	ID       []byte
	Graph    *drgraph.Graph
	NodeSize int
	Replica  data.ReadWriter
	TreeD    gomerkle.Tree
}

// NewProver creates and returns a new Prover with specified id, Graph, and nodeSize.
func NewProver(id string, g *drgraph.Graph, nodeSize int) *Prover {
	return &Prover{
		ID:       []byte(id),
		Graph:    g,
		NodeSize: nodeSize,
	}
}

// MakeProvers creates and returns numberOfProvers Provers.
func MakeProvers(id string, G *drgraph.Graph, nodeSize int, numberOfProvers int) []*Prover {
	provers := make([]*Prover, numberOfProvers)
	for i := 0; i < numberOfProvers; i++ {
		provers[i] = NewProver(id, G, nodeSize)
	}
	return provers
}

// Setup TODO
func (p *Prover) Setup(data data.ReadWriter) ([]byte, error) {
	p.TreeD = p.Graph.MerkleTree(data, p.NodeSize)
	p.seal(data)
	p.Replica = data

	return p.Graph.Commit(p.Replica, p.NodeSize), nil
}

// Prove TODO
func (p *Prover) Prove(challenge int) *Proof {
	challenge = challenge % p.Graph.Nodes

	proof := Proof{}
	treeD := p.TreeD
	treeR := p.Graph.MerkleTree(p.Replica, p.NodeSize)

	// 1. Get challenged replica node
	d, err := crypto.DataAtNode(p.Replica, challenge+1, p.NodeSize)
	if err != nil {
		panic(err)
	}
	proof.ReplicaNode = &crypto.MerkleProof{
		Data: d,
		Hash: treeR.GetLeaf(challenge),
		Path: treeR.GetProof(challenge),
	}

	// 2. Get predecessor of challenged node
	proof.ReplicaParents = make(map[int]*crypto.MerkleProof)
	for _, pred := range p.Graph.Pred[challenge+1] {
		d, err = crypto.DataAtNode(p.Replica, pred, p.NodeSize)
		if err != nil {
			panic(err)
		}

		proof.ReplicaParents[pred] = &crypto.MerkleProof{
			Data: d,
			Hash: treeR.GetLeaf(pred - 1),
			Path: treeR.GetProof(pred - 1),
		}
	}

	d, err = p.UnsealBlockRead(challenge + 1)
	if err != nil {
		panic(err)
	}
	// d, err = crypto.DataAtNode(p.Data, challenge+1, p.NodeSize)
	// if err != nil {
	// 	panic(err)
	// }

	// 3. Get original replica node
	proof.Node = &crypto.MerkleProof{
		Data: d,
		Hash: treeD.GetLeaf(challenge),
		Path: treeD.GetProof(challenge),
	}

	return &proof
}
