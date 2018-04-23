package drg

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/filecoin-project/paper-porep-drg/go-porep/drg/crypto"
	"github.com/filecoin-project/paper-porep-drg/go-porep/drg/drgraph"
)

type Context struct {
	NodeSize int
	Graph    *drgraph.Graph
	Prover   *Prover
	Verifier *Verifier
}

func RunSetup(nodeSize int, data []byte) Context {
	// Public params
	n := len(data) / nodeSize
	G := drgraph.New(n)

	// Verifier - setup
	veri := NewVerifier(G, nodeSize)
	veri.Setup(data)

	// Prover - setup
	prov := NewProver("miner1", G, nodeSize)
	_, commR := prov.Setup(data)

	// Prover sends setup to Verifier
	// TODO(@ben): do we need this?
	veri.CommR = commR

	return Context{nodeSize, G, prov, veri}
}

func RunAllInteractions(context Context) (bool, error) {
	for challenge := 0; challenge < context.Graph.Nodes; challenge++ {
		res, err := RunInteraction(context, challenge)
		if err != nil {
			return res, err
		}
		if !res {
			panic("false proof must return error")
		}
	}
	return true, nil
}

func RunInteraction(Context Context, challenge int) (bool, error) {
	proof := Context.Prover.Prove(challenge)
	res, err := Context.Verifier.Verify(Context.Prover.ID, challenge, proof)

	if err != nil {
		fmt.Println("Prover setup:")
		fmt.Println("\tdata:\t", crypto.DataAtNode(Context.Prover.Data, 50+1, Context.NodeSize))
		fmt.Println("\trepl:\t", crypto.DataAtNode(Context.Prover.Replica, 50+1, Context.NodeSize))

		var ciphertexts []byte
		for _, pred := range Context.Prover.Graph.Pred[challenge+1] {
			ciphertexts = append(ciphertexts, crypto.DataAtNode(Context.Prover.Replica, pred, Context.NodeSize)...)
		}
		fmt.Println("\tciphertexts:\t", ciphertexts)
		fmt.Println("\tkeygen:\t", crypto.Kdf(ciphertexts))

		fmt.Println("challenge", challenge)
		fmt.Println("Prover proof:")
		fmt.Println("\tdata:\t", proof.ErasureNode.Data)
		fmt.Println("\trepl:\t", proof.ReplicaNode.Data)

		fmt.Println("Verifier verify:")
		var ciphertextsV []byte
		for _, mProof := range proof.ReplicaParents {
			ciphertextsV = append(ciphertextsV, mProof.Data...)
		}
		fmt.Println("\tciphertexts:\t", ciphertextsV)
		keyV := crypto.Kdf(ciphertextsV)
		fmt.Println("\tkeygen:\t", keyV)
		fmt.Println("\tdecd:\t", crypto.Dec(Context.Prover.ID, keyV, proof.ReplicaNode.Data))

	}

	return res, err
}

func randomData(size uint) []byte {
	out := make([]byte, size)
	// rand.Seed(seed)
	_, err := rand.Read(out)
	if err != nil {
		panic(err)
	}
	return out
}

func TestSealUnseal(t *testing.T) {
	// Number of nodes in the graph
	n := 3
	// Amount of data to encode at each node
	nodeSize := 16
	// Data to be sealed
	raw := []byte("111111111111111122222222222222223333333333333333")

	// Generate a graph
	G := drgraph.New(n)

	// Our sealer
	s := NewProver("miner1", G, nodeSize)

	// Erasure code and seal
	coded := G.ErasureCoding(raw, nodeSize)
	seal := s.seal(coded)

	// Test length of seal
	expectedLen := n * nodeSize
	if expectedLen != len(seal) {
		t.Fatal("Seal is not of the right size")
	}

	// Test length of unseal
	s.Replica = seal
	unseal := s.Unseal(0)
	if expectedLen != len(unseal) {
		t.Fatal("Seal is of size", len(unseal), "while data is of size", expectedLen)
	}

	if !bytes.Equal(raw, unseal) {
		t.Fatalf("Expected %x to equal %x", raw, unseal)
	}

	// check that coded equals to unseal
	for i := range coded {
		if coded[i] != unseal[i] {
			t.Fatal("Byte at index", i, "is", unseal[i], "instead of", coded[i])
		}
	}
}

// make sure things don't get optimized away
var unseal []byte

func benchmarkUnseal(n, nodeSize int, par float64, b *testing.B) {
	// Data to be sealed
	raw := randomData(1024 * 1024)

	// Generate a graph
	G := drgraph.New(n)

	// Our sealer
	s := NewProver("miner1", G, nodeSize)

	// Erasure code and seal
	coded := G.ErasureCoding(raw, nodeSize)
	seal := s.seal(coded)

	// Test length of seal
	expectedLen := n * nodeSize
	if expectedLen != len(seal) {
		b.Fatal("Seal is not of the right size")
	}

	b.ResetTimer()

	s.Replica = seal
	for i := 0; i < b.N; i++ {
		// Test length of unseal
		unseal = s.Unseal(par)
	}
}

// Parallel, num cpus
func BenchmarkUnsealParN3NodeSize16(b *testing.B)     { benchmarkUnseal(3, 16, 0, b) }
func BenchmarkUnsealParN30NodeSize16(b *testing.B)    { benchmarkUnseal(30, 16, 0, b) }
func BenchmarkUnsealParN300NodeSize16(b *testing.B)   { benchmarkUnseal(300, 16, 0, b) }
func BenchmarkUnsealParN10000NodeSize16(b *testing.B) { benchmarkUnseal(10000, 16, 0, b) }

// single threaded
func BenchmarkUnsealSingleN3NodeSize16(b *testing.B)     { benchmarkUnseal(3, 16, 1, b) }
func BenchmarkUnsealSingleN30NodeSize16(b *testing.B)    { benchmarkUnseal(30, 16, 1, b) }
func BenchmarkUnsealSingleN300NodeSize16(b *testing.B)   { benchmarkUnseal(300, 16, 1, b) }
func BenchmarkUnsealSingleN10000NodeSize16(b *testing.B) { benchmarkUnseal(10000, 16, 1, b) }

func Test3Nodes(t *testing.T) {
	context := RunSetup(16, []byte("AAAAAAAAAAAAAAAA9999999999999999ZZZZZZZZZZZZZZZZ"))
	res, err := RunAllInteractions(context)

	if err != nil {
		t.Fatal(err)
	}
	if !res {
		t.Fatal("proof failed")
	}
}

func Test100Nodes(t *testing.T) {
	seed := time.Now().UnixNano()
	rand.Seed(seed)
	fmt.Println("graph seed:", seed)

	nodeSize := 16
	data := randomData(uint(nodeSize) * 100)
	context := RunSetup(nodeSize, data)
	res, err := RunAllInteractions(context)

	if err != nil {
		t.Fatal(err)
	}
	if !res {
		t.Fatal("proof failed")
	}
}

func Test300Nodes(t *testing.T) {
	nodeSize := 16
	data := randomData(uint(nodeSize) * 300)
	context := RunSetup(nodeSize, data)
	res, err := RunAllInteractions(context)

	if err != nil {
		t.Fatal(err)
	}
	if !res {
		t.Fatal("proof failed")
	}
}
