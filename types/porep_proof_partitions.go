package types

import "fmt"

type PoRepProofPartitions int

const (
	UnknownPoRepProofPartitions = PoRepProofPartitions(iota)
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
func NewPoRepProofPartitions(numPartitions int) PoRepProofPartitions {
	switch numPartitions {
	case 2:
		return TwoPoRepProofPartitions
	default:
		panic(fmt.Sprintf("unexpected value %v", numPartitions))
	}
}
