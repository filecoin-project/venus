package types

type PoStProofPartitions int

const (
	UnknownPoStProofPartitions = PoStProofPartitions(iota)
	OnePoStProofPartition
)

func (p PoStProofPartitions) Int() int {
	switch p {
	case OnePoStProofPartition:
		return 1
	default:
		return 0
	}
}

func (p PoStProofPartitions) ProofLen() int {
	switch p {
	case OnePoStProofPartition:
		return SinglePartitionProofLen
	default:
		return 0
	}
}

func NewPoStProofPartitions(partitions int) PoStProofPartitions {
	switch partitions {
	case 1:
		return OnePoStProofPartition
	default:
		return UnknownPoStProofPartitions
	}
}
