package types

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
		return 0
	}
}

// ProofLen returns an integer representing the number of bytes in a PoRep proof
// created with this number of partitions.
func (p PoRepProofPartitions) ProofLen() int {
	switch p {
	case TwoPoRepProofPartitions:
		return SinglePartitionProofLen * 2
	default:
		return 0
	}
}

// NewPoRepProofPartitions produces the PoRepProofPartitions corresponding to
// the provided integer.
func NewPoRepProofPartitions(partitions int) PoRepProofPartitions {
	switch partitions {
	case 2:
		return TwoPoRepProofPartitions
	default:
		return UnknownPoRepProofPartitions
	}
}
