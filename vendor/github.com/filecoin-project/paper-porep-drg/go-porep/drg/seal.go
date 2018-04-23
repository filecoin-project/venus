package drg

import (
	"math"
	"runtime"
	"sync"

	"github.com/filecoin-project/paper-porep-drg/go-porep/drg/crypto"
)

func (p *Prover) seal(data []byte) []byte {
	C := map[int][]byte{}

	var ciphertext []byte
	for i := 0; i < p.Graph.Nodes; i++ {
		ciphertext = append(ciphertext, p.recursiveSeal(C, data, i+1)...)
	}

	return ciphertext
}

func (p *Prover) recursiveSeal(C map[int][]byte, data []byte, node int) []byte {
	// recursive implementation - not necessarily the best impl
	if _, ok := C[node]; !ok {
		// seal the parents
		var ciphertexts []byte
		for _, parent := range p.Graph.Pred[node] {
			ciphertexts = append(ciphertexts, p.recursiveSeal(C, data, parent)...)
		}

		// generate the key
		key := crypto.Kdf(ciphertexts)

		// finally seal this
		C[node] = crypto.Enc(p.ID, key, crypto.DataAtNode(data, node, p.NodeSize))
	}

	return C[node]
}

// passing cpus = 0, uses the current cpu count
// anything else fixes the parallelism at that number
func (p *Prover) Unseal(cpus float64) []byte {
	n := p.Graph.Nodes
	result := make([][]byte, n)

	if cpus == 0 {
		cpus = math.Min(float64(runtime.NumCPU()), float64(n))
	}

	var wg sync.WaitGroup
	chunkSize := int(math.Max(float64(n)/cpus, 1))

	for i := 0; i < n; i += chunkSize {
		wg.Add(1)
		go p.unsealWorker(&wg, n, p.Replica, result, i, chunkSize)
	}

	wg.Wait()

	// flatten slices
	var out []byte
	for _, part := range result {
		out = append(out, part...)
	}
	return out
}

func (p *Prover) unsealWorker(wg *sync.WaitGroup, n int, seal []byte, out [][]byte, idx, chunkSize int) {
	for i := idx; i < idx+chunkSize; i++ {
		if i >= n {
			break
		}

		out[i] = p.UnsealBlock(i + 1)
	}

	wg.Done()
}

func (p *Prover) UnsealBlock(v int) []byte {
	var ciphertexts []byte
	for _, parent := range p.Graph.Pred[v] {
		ciphertexts = append(ciphertexts, crypto.DataAtNode(p.Replica, parent, p.NodeSize)...)
	}

	key := crypto.Kdf(ciphertexts)
	plaintext := crypto.Dec(p.ID, key, crypto.DataAtNode(p.Replica, v, p.NodeSize))
	return plaintext
}
