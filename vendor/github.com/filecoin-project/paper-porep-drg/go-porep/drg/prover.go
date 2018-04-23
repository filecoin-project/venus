package drg

import (
	"github.com/onrik/gomerkle"

	"github.com/filecoin-project/paper-porep-drg/go-porep/drg/crypto"
	"github.com/filecoin-project/paper-porep-drg/go-porep/drg/drgraph"
)

type Prover struct {
	ID       []byte
	Graph    *drgraph.Graph
	NodeSize int
	Replica  []byte
	Data     []byte
}

type Proof struct {
	ReplicaNode    *crypto.MerkleProof
	ReplicaParents map[int]*crypto.MerkleProof
	ErasureNode    *crypto.MerkleProof
}

func NewProver(id string, g *drgraph.Graph, size int) *Prover {
	return &Prover{
		ID:       []byte(id),
		Graph:    g,
		NodeSize: size,
	}
}

func (p *Prover) Setup(data []byte) (gomerkle.Tree, []byte) {
	p.Data = p.Graph.ErasureCoding(data, p.NodeSize)
	p.Replica = p.seal(p.Data)

	return p.Graph.Commit(p.Replica, p.NodeSize)
}

func (p *Prover) Prove(challenge int) *Proof {
	challenge = challenge % p.Graph.Nodes

	proof := Proof{}
	treeE := p.Graph.MerkleTree(p.Data, p.NodeSize)
	treeR := p.Graph.MerkleTree(p.Replica, p.NodeSize)

	// 1. Get challenged replica node
	proof.ReplicaNode = &crypto.MerkleProof{
		Data: crypto.DataAtNode(p.Replica, challenge+1, p.NodeSize),
		Hash: treeR.GetLeaf(challenge),
		Path: treeR.GetProof(challenge),
	}

	// 2. Get predecessor of challenged node
	proof.ReplicaParents = make(map[int]*crypto.MerkleProof)
	for _, pred := range p.Graph.Pred[challenge+1] {
		proof.ReplicaParents[pred] = &crypto.MerkleProof{
			Data: crypto.DataAtNode(p.Replica, pred, p.NodeSize),
			Hash: treeR.GetLeaf(pred - 1),
			Path: treeR.GetProof(pred - 1),
		}
	}

	// 3. Get original replica node
	proof.ErasureNode = &crypto.MerkleProof{
		Data: crypto.DataAtNode(p.Data, challenge+1, p.NodeSize),
		Hash: treeE.GetLeaf(challenge),
		Path: treeE.GetProof(challenge),
	}

	return &proof
}
