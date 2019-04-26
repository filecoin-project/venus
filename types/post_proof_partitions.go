package types

type PoStProofPartitions int

const (
	UnknownPoStProofPartitions = PoStProofPartitions(iota)
	OnePoStProofPartition
)

// Int returns an integer representing the number of PoSt partitions
func (p PoStProofPartitions) Int() int {
	switch p {
	case OnePoStProofPartition:
		return 1
	default:
		return 0
	}
}

// ProofLen returns an integer representing the number of bytes in a PoSt proof
// created with this number of partitions.
func (p PoStProofPartitions) ProofLen() int {
	switch p {
	case OnePoStProofPartition:
		return SinglePartitionProofLen
	default:
		return 0
	}
}

// NewPoStProofPartitions produces the PoStProofPartitions corresponding to the
// provided integer.
func NewPoStProofPartitions(partitions int) PoStProofPartitions {
	switch partitions {
	case 1:
		return OnePoStProofPartition
	default:
		return UnknownPoStProofPartitions
	}
}
