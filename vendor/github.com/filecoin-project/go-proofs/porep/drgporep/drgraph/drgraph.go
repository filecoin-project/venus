package drgraph

import (
	"crypto/sha256"
	"encoding/json"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"sort"

	"github.com/filecoin-project/go-proofs/crypto"
	"github.com/filecoin-project/go-proofs/data"
	"github.com/onrik/gomerkle"
)

// Tuple is a From/To pair of nodes.
type Tuple struct {
	From int
	To   int
}

// Graph is a collection of nodes and their predecessors (i.e. a DAG).
type Graph struct {
	Nodes int
	Pred  map[int][]int
}

// ToJSON returns JSON representing Graph, g.
func (g *Graph) ToJSON() string {
	b, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		panic(err)
	}

	return string(b)
}

// FromJSON populates Graph, g, with JSON supplied in data.
func (g *Graph) FromJSON(data string) {
	if err := json.Unmarshal([]byte(data), &g); err != nil {
		panic(err)
	}
}

// DRSample TODO
// TODO(ben): should we have a power of two size graph?
func (g *Graph) DRSample(n int) {
	if n < 2 {
		panic("graph too small")
	}
	g.Nodes = n
	g.AddEdge(2, 1)

	for v := 3; v < g.Nodes; v++ {
		g.AddEdge(v, v-1)
		g.AddEdge(v, getParent(v))
	}
}

// BucketSampling TODO
// TODO(nicola): Make sure u != j
func BucketSampling(gDash *Graph, n int, m int) *Graph {
	// For each (u, v) in G' bucket sample their edges
	// add i, j to G
	g := &Graph{}
	g.Pred = make(map[int][]int)
	g.Nodes = n

	cache := make(map[Tuple]bool)
	for v := 1; v <= gDash.Nodes; v++ {
		for _, u := range gDash.Pred[v] {
			i := ((u - 1) / m) + 1
			j := ((v - 1) / m) + 1
			if i != j && !cache[Tuple{i, j}] {
				cache[Tuple{i, j}] = true
				g.AddEdge(i, j)
			}
		}
	}

	// reorder
	for v := 1; v <= g.Nodes; v++ {
		sort.Ints(g.Pred[v])
	}

	// for v := 1; v <= g.Nodes; v++ {
	// 	fmt.Println(v, g.Pred[v])
	// }

	return g
}

func getParent(v int) int {
	j := randomRange(1, floorLog2(v)+1)
	g := min(v-1, int(math.Pow(2, float64(j))))
	halfG := g / 2.0
	r := randomRange(max(halfG, 2), g)
	return v - r
}

// New creates and returns a new Graph.
func New() *Graph {
	G := &Graph{}
	G.Pred = make(map[int][]int)

	return G
}

// NewDRSample TODO
func NewDRSample(n int) *Graph {
	G := &Graph{}
	G.Pred = make(map[int][]int)
	G.DRSample(n)

	return G
}

// NewFromStdin creates and returns a Graph initialized with JSON read from stdin.
func NewFromStdin(data io.Reader) *Graph {
	d, err := ioutil.ReadAll(data)
	if err != nil {
		panic(err)
	}

	return NewFromJSON(string(d))
}

// NewFromJSON creates and returns a Graph initialized with JSON supplied in data.
func NewFromJSON(data string) *Graph {
	g := New()
	g.FromJSON(data)
	return g
}

// NewParallel TODO
func NewParallel(n int) *Graph {
	G := &Graph{}
	G.Pred = make(map[int][]int)
	G.Nodes = n

	return G
}

// NewBucketSample TODO
func NewBucketSample(n int, m int) *Graph {
	gDash := NewDRSample(n * m)
	g := BucketSampling(gDash, n, m)
	// fmt.Println(gDash)
	// fmt.Println(g)
	return g
}

// AddEdge adds an edge from node u to node v in Graph, g.
func (g *Graph) AddEdge(u int, v int) {
	g.Pred[v] = append(g.Pred[v], u)
}

// MerkleTree builds and returns a merkle tree from data.
func (g *Graph) MerkleTree(data data.ReadWriter, nodeSize int) gomerkle.Tree {
	tree := gomerkle.NewTree(sha256.New())
	for i := 0; i < g.Nodes; i++ {
		d, err := crypto.DataAtNode(data, i+1, nodeSize)
		if err != nil {
			panic(err)
		}
		tree.AddData(d)
	}

	err := tree.Generate()
	if err != nil {
		panic(err)
	}

	return tree
}

// Commit returns the commitment hash for data
func (g *Graph) Commit(data data.ReadWriter, nodeSize int) []byte {
	tree := g.MerkleTree(data, nodeSize)
	comm := tree.Root()
	return comm
}

// ExpectedSize returns the total size of all nodes in Graph, g.
func (g *Graph) ExpectedSize(nodeSize int) int {
	return g.Nodes * nodeSize
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

func randomRange(min int, max int) int {
	if max-min == 0 {
		return min
	}
	return rand.Intn(max-min) + min
}
