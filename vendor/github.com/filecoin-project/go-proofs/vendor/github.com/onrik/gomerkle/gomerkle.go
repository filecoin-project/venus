package gomerkle

import (
	"bytes"
	"errors"
	"hash"
)

// Proof presents merkle tree proof
type Proof []map[string][]byte

// Node presents merkle tree node
type Node struct {
	Hash  []byte
	Left  *Node
	Right *Node
}

type block struct {
	data   []byte
	hashed bool
}

// NewNode creates a node given a hash function and data to hash
func NewNode(h hash.Hash, block []byte) (Node, error) {
	if block == nil {
		return Node{}, nil
	}
	if h == nil {
		return Node{Hash: block}, nil
	}

	defer h.Reset()
	_, err := h.Write(block[:])
	if err != nil {
		return Node{}, err
	}
	return Node{Hash: h.Sum(nil)}, nil
}

// Tree presents merkle tree
type Tree struct {
	hasher hash.Hash
	blocks []block

	// All nodes, linear
	Nodes []Node
	// Points to each level in the node. The first level contains the root node
	Levels [][]Node
}

// NewTree creates tree
func NewTree(hasher hash.Hash) Tree {
	return Tree{
		hasher: hasher,
		blocks: []block{},
	}
}

// AddData add data to the tree
func (tree *Tree) AddData(data ...[]byte) {
	blocks := make([]block, len(data))
	for i := range data {
		blocks[i] = block{data[i], false}
	}
	tree.blocks = append(tree.blocks, blocks...)
}

// AddHash add hashed data to the tree
func (tree *Tree) AddHash(data ...[]byte) {
	blocks := make([]block, len(data))
	for i := range data {
		blocks[i] = block{data[i], true}
	}
	tree.blocks = append(tree.blocks, blocks...)
}

func (tree *Tree) hash(data []byte) []byte {
	tree.hasher.Write(data)
	defer tree.hasher.Reset()

	return tree.hasher.Sum(nil)
}

// Root returns the root node of the tree, if available, else nil
func (tree *Tree) Root() []byte {
	if tree.Nodes == nil {
		return nil
	}

	return tree.Levels[0][0].Hash
}

// GetLeaf returns leaf hash
func (tree *Tree) GetLeaf(index int) []byte {
	if tree.Levels == nil {
		return nil
	}

	return tree.Levels[len(tree.Levels)-1][index].Hash
}

// GetProof generates proof
func (tree *Tree) GetProof(index int) Proof {
	proof := Proof{}
	var siblingIndex int
	var siblingPosition string

	for i := len(tree.Levels) - 1; i > 0; i-- {
		levelLen := len(tree.Levels[i])
		if (index == levelLen-1) && (levelLen%2 == 1) {
			index = int(index / 2)
			continue
		}

		if index%2 == 0 {
			siblingIndex = index + 1
			siblingPosition = "right"
		} else {
			siblingIndex = index - 1
			siblingPosition = "left"
		}

		proof = append(proof, map[string][]byte{
			siblingPosition: tree.Levels[i][siblingIndex].Hash,
		})
		index = int(index / 2)
	}

	return proof
}

// VerifyProof verify proof for value
func (tree *Tree) VerifyProof(proof Proof, root, value []byte) bool {
	proofHash := value

	for _, p := range proof {
		if sibling, exist := p["left"]; exist {
			proofHash = tree.hash(append(sibling, proofHash...))
		} else if sibling, exist := p["right"]; exist {
			proofHash = tree.hash(append(proofHash, sibling...))
		} else {
			return false
		}
	}

	return bytes.Equal(root, proofHash)
}

// GetNodesAtHeight returns all nodes at a given height, where height 1 returns a 1-element
// slice containing the root node, and a height of tree.Height() returns
// the leaves
func (tree *Tree) GetNodesAtHeight(h uint64) []Node {
	if tree.Levels == nil || h == 0 || h > uint64(len(tree.Levels)) {
		return nil
	}

	return tree.Levels[h-1]
}

// Height returns height of the tree
func (tree *Tree) Height() uint64 {
	return uint64(len(tree.Levels))
}

// Generate generates the tree nodes
func (tree *Tree) Generate() error {
	blockCount := uint64(len(tree.blocks))
	if blockCount == 0 {
		return errors.New("Empty tree")
	}
	height, nodeCount := calculateHeightAndNodeCount(blockCount)
	levels := make([][]Node, height)
	nodes := make([]Node, nodeCount)

	var err error
	// Create the leaf nodes
	for i, block := range tree.blocks {
		if block.hashed {
			nodes[i], err = NewNode(nil, block.data)
		} else {
			nodes[i], err = NewNode(tree.hasher, block.data)
		}
		if err != nil {
			return err
		}
	}
	levels[height-1] = nodes[:len(tree.blocks)]

	if blockCount > 1 {
		// Create each node level
		current := nodes[len(tree.blocks):]
		for i := height - 1; i > 0; i-- {
			below := levels[i]
			wrote, err := tree.generateNodeLevel(below, current, tree.hasher)
			if err != nil {
				return err
			}
			levels[i-1] = current[:wrote]
			current = current[wrote:]
		}
	}

	tree.Nodes = nodes
	tree.Levels = levels
	return nil
}

// Creates all the non-leaf nodes for a certain height. The number of nodes
// is calculated to be 1/2 the number of nodes in the lower rung.  The newly
// created nodes will reference their Left and Right children.
// Returns the number of nodes added to current
func (tree *Tree) generateNodeLevel(below []Node, current []Node, h hash.Hash) (uint64, error) {
	h.Reset()
	size := h.Size()
	data := make([]byte, size*2)
	end := (len(below) + (len(below) % 2)) / 2
	for i := 0; i < end; i++ {
		// Concatenate the two children hashes and hash them, if both are
		// available, otherwise reuse the hash from the lone left node
		node := Node{}
		ileft := 2 * i
		iright := 2*i + 1
		left := &below[ileft]
		var right *Node
		if len(below) > iright {
			right = &below[iright]
		}
		if right == nil {
			b := data[:size]
			copy(b, left.Hash)
			node = Node{Hash: b}
		} else {
			copy(data[:size], below[ileft].Hash)
			copy(data[size:], below[iright].Hash)
			var err error
			node, err = NewNode(h, data)
			if err != nil {
				return 0, err
			}
		}
		// Point the new node to its children and save
		node.Left = left
		node.Right = right
		current[i] = node

		// Reset the data slice
		data = data[:]
	}
	return uint64(end), nil
}

// Returns the height and number of nodes in an unbalanced binary tree given number of leaves
func calculateHeightAndNodeCount(leaves uint64) (height, nodeCount uint64) {
	height = calculateTreeHeight(leaves)
	nodeCount = calculateNodeCount(height, leaves)
	return
}

// Calculates the number of nodes in a binary tree unbalanced strictly on
// the right side.  Height is assumed to be equal to
// calculateTreeHeight(size)
func calculateNodeCount(height, size uint64) uint64 {
	if isPowerOfTwo(size) {
		return 2*size - 1
	}
	count := size
	prev := size
	i := uint64(1)
	for ; i < height; i++ {
		next := (prev + (prev % 2)) / 2
		count += next
		prev = next
	}
	return count
}

// Returns the height of a full, complete binary tree given nodeCount nodes
func calculateTreeHeight(nodeCount uint64) uint64 {
	if nodeCount == 0 {
		return 0
	} else if nodeCount == 1 {
		return 1
	} else {
		return log2(nextPowerOfTwo(nodeCount)) + 1
	}
}

// Returns true if n is a power of 2
func isPowerOfTwo(n uint64) bool {
	// http://graphics.stanford.edu/~seander/bithacks.html#DetermineIfPowerOf2
	return n != 0 && (n&(n-1)) == 0
}

// Returns the next highest power of 2 above n, if n is not already a
// power of 2
func nextPowerOfTwo(n uint64) uint64 {
	if n == 0 {
		return 1
	}
	// http://graphics.stanford.edu/~seander/bithacks.html#RoundUpPowerOf2
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n |= n >> 32
	n++
	return n
}

// Returns log2(n)
func log2(x uint64) uint64 {
	if x == 0 {
		return 0
	}
	ct := uint64(0)
	for x != 0 {
		x >>= 1
		ct++
	}
	return ct - 1
}
