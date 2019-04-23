package types

import (
	"fmt"

	"github.com/pkg/errors"
)

// PoRepProofPartitions represents the number of partitions used when creating a
// PoRep proof, and impacts the size of the proof.
type PoRepProofPartitions int

const (
	// UnknownPoRepProofPartitions is an opaque value signaling that an unknown number
	// of partitions were used when creating a PoRep proof in test mode.
	UnknownPoRepProofPartitions = PoRepProofPartitions(iota)

	// TwoPoRepProofPartitions indicates that two partitions were used to create a PoRep proof.
	TwoPoRepProofPartitions
)

// Int returns an integer representing the number of PoRep partitions
func (p PoRepProofPartitions) Int() int {
	switch p {
	case TwoPoRepProofPartitions:
		return 2
	default:
		panic(fmt.Sprintf("unexpected value %v", p))
	}
}

// ProofLen returns an integer representing the number of bytes in a PoRep proof
// created with this number of partitions.
func (p PoRepProofPartitions) ProofLen() int {
	switch p {
	case TwoPoRepProofPartitions:
		return SinglePartitionProofLen * 2
	default:
		panic(fmt.Sprintf("unexpected value %v", p))
	}
}

// NewPoRepProofPartitions produces the PoRepProofPartitions corresponding to
// the provided integer.
func NewPoRepProofPartitions(numPartitions int) (PoRepProofPartitions, error) {
	switch numPartitions {
	case 2:
		return TwoPoRepProofPartitions, nil
	default:
		return UnknownPoRepProofPartitions, errors.Errorf("unexpected value %v", numPartitions)
	}
}
