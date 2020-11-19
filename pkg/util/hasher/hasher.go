package hasher

import (
	"crypto/sha256"
	"encoding/binary"
)

// Hasher takes in an ordered series of data and hashes it together
type Hasher struct {
	toHash [][]byte
}

// NewHasher creates a new Hasher
func NewHasher() *Hasher {
	return &Hasher{
		toHash: make([][]byte, 0),
	}
}

// Int adds a uint64 to the data to be hashed
func (h *Hasher) Int(i uint64) {
	intData := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(intData, i)
	h.toHash = append(h.toHash, intData[:n])
}

// Bytes adds a byte slice to the data to be hashed
func (h *Hasher) Bytes(bs []byte) {
	h.toHash = append(h.toHash, bs[:])
}

// Hash returns the hash value of the provided data
func (h *Hasher) Hash() []byte {
	var accum []byte
	for _, s := range h.toHash {
		accum = append(accum, s...)
	}
	hash := sha256.Sum256(accum)

	// clear the hasher for reuse
	h.toHash = make([][]byte, 0)
	return hash[:]
}
