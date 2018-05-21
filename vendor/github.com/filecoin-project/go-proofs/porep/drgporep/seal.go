package drgporep

import (
	"math"
	"runtime"
	"sync"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-proofs/crypto"
	"github.com/filecoin-project/go-proofs/data"
)

// seal seals the data and overwrites the original data.
func (p *Prover) seal(data data.ReadWriter) {
	// list to keep track of which parts have been encrypted already
	// this is zero based indexing, but nodes are 1 based indexed
	C := make([]bool, p.Graph.Nodes)

	// iterate in reverse, as this means less recursion
	for i := p.Graph.Nodes - 1; i > -1; i-- {
		if err := p.recursiveSeal(C, data, i+1); err != nil {
			panic(err)
		}
	}
}

// recursiveSeal, seals the data of the passed in node and all its parents using the data in input,
// overwriting unsealed data.
func (p *Prover) recursiveSeal(C []bool, data data.ReadWriter, node int) error {
	// recursive implementation - not necessarily the best impl
	if !C[node-1] {
		parents := p.Graph.Pred[node]

		// -- seal all parents of this node
		for _, parent := range parents {
			if err := p.recursiveSeal(C, data, parent); err != nil {
				return err
			}
		}

		// -- create sealing key for this node
		//
		// using the parents, and output, which holds
		// the already sealed data for all parents, as we have sealed those above, or in a previous
		// iteration of this call
		key, err := createKey(parents, data, p.NodeSize)
		if err != nil {
			return err
		}

		// -- seal this node

		// read the original data out
		dataAtNode, err := crypto.DataAtNode(data, node, p.NodeSize)
		if err != nil {
			return err
		}

		// encrypt the data
		out := crypto.Enc(p.ID, key, dataAtNode)

		// write the sealed data into our output
		offset := crypto.DataAtNodeOffset(node, p.NodeSize)
		err = data.DataAt(uint64(offset), uint64(len(out)), func(b []byte) error {
			copy(b, out)
			return nil
		})
		if err != nil {
			return err
		}

		// mark this node as sealed
		C[node-1] = true
	}

	return nil
}

// Unseal unseals the data in p.Replica and writes the result into output.
//
// passing cpus = 0, uses the current cpu count
// anything else fixes the parallelism at that number
func (p *Prover) Unseal(cpus float64, output data.ReadWriter) {
	n := p.Graph.Nodes

	if cpus == 0 {
		cpus = math.Min(float64(runtime.NumCPU()), float64(n))
	}

	var wg sync.WaitGroup
	chunkSize := int(math.Max(float64(n)/cpus, 1))

	for i := 0; i < n; i += chunkSize {
		wg.Add(1)
		go p.unsealWorker(&wg, n, output, i, chunkSize)
	}

	wg.Wait()
}

// unsealWorker handles unsealing of a list of blocks.
func (p *Prover) unsealWorker(wg *sync.WaitGroup, n int, out data.ReadWriter, idx, chunkSize int) {
	for i := idx; i < idx+chunkSize; i++ {
		if i >= n {
			break
		}
		if err := p.UnsealBlock(i+1, out); err != nil {
			panic(err)
		}
	}

	wg.Done()
}

// UnsealBlock unseals a single node v from p.Replica and writes the result into output.
func (p *Prover) UnsealBlock(v int, output data.ReadWriter) error {
	plaintext, err := p.UnsealBlockRead(v)
	if err != nil {
		return err
	}

	offset := crypto.DataAtNodeOffset(v, p.NodeSize)
	err = output.DataAt(uint64(offset), uint64(len(plaintext)), func(b []byte) error {
		copy(b, plaintext)
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to write plaintext")
	}

	return nil
}

// UnsealBlockRead unseals a single node v from p.Replica and returns the result.
func (p *Prover) UnsealBlockRead(v int) ([]byte, error) {
	key, err := createKey(p.Graph.Pred[v], p.Replica, p.NodeSize)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create key")
	}

	d, err := crypto.DataAtNode(p.Replica, v, p.NodeSize)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve data for node")
	}

	plaintext := crypto.Dec(p.ID, key, d)
	return plaintext, nil
}

func createKey(parents []int, sealed data.ReadWriter, nodeSize int) ([]byte, error) {
	// ciphertexts will become a buffer of the layout
	// encodedParentNode1 | encodedParentNode1 | ...
	ciphertexts := make([]byte, len(parents)*nodeSize)

	for i, parent := range parents {
		offset := crypto.DataAtNodeOffset(parent, nodeSize)
		// read the data into the ciphertext byte slice at the right position
		err := sealed.DataAt(uint64(offset), uint64(nodeSize), func(b []byte) error {
			copy(ciphertexts[i*nodeSize:(i+1)*nodeSize], b)
			return nil
		})
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read: [%d, %d]", i, offset)
		}
	}

	// generate the key
	return crypto.Kdf(ciphertexts), nil
}
