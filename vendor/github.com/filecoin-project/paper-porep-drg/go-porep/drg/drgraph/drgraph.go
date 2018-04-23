package drgraph

import (
	"crypto/sha256"
	"github.com/filecoin-project/paper-porep-drg/go-porep/drg/crypto"
	"github.com/onrik/gomerkle"
	"math"
)

type Graph struct {
	Nodes int
	Pred  map[int][]int
}

func (g *Graph) DRSample(n int) {
	if n < 2 {
		panic("graph too small")
	}
	g.Nodes = n
	g.AddEdge(1, 2)

	for v := 3; v < g.Nodes; v++ {
		g.AddEdge(v, v-1)
		g.AddEdge(v, getParent(v))
	}

}

func (g *Graph) AddEdge(u int, v int) {
	g.Pred[v] = append(g.Pred[v], u)
}

func (g *Graph) MerkleTree(data []byte, nodeSize int) gomerkle.Tree {
	tree := gomerkle.NewTree(sha256.New())
	for i := 0; i < g.Nodes; i++ {
		tree.AddData(crypto.DataAtNode(data, i+1, nodeSize))
	}

	err := tree.Generate()
	if err != nil {
		panic(err)
	}

	return tree
}

func (g *Graph) Commit(data []byte, nodeSize int) (gomerkle.Tree, []byte) {
	tree := g.MerkleTree(data, nodeSize)
	comm := tree.Root()
	return tree, comm
}

func (g *Graph) ErasureCoding(data []byte, nodeSize int) []byte {
	// Erasure coding
	// TODO: this needs to go out to somewhere and not be a buffer
	coded := make([]byte, g.Nodes*nodeSize)
	copy(coded, data)

	return coded
}

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func floorLog2(i int) int {
	return int(math.Log2(float64(i)) + 0.5)
}

func getParent(v int) int {
	j := crypto.RandomRange(1, floorLog2(v)+1)
	g := min(v-1, int(math.Pow(2, float64(j))))
	halfG := g / 2.0
	r := crypto.RandomRange(max(halfG, 2), g)
	return v - r
}

func New(n int) *Graph {
	G := &Graph{}
	G.Pred = make(map[int][]int)
	G.DRSample(n)

	return G
}
