package types

type PoRepProofPartitions int

const (
	UnknownPoRepProofPartitions = PoRepProofPartitions(iota)
	TwoPoRepProofPartitions
)

func (p PoRepProofPartitions) Uint8() uint8 {
	switch p {
	case TwoPoRepProofPartitions:
		return 2
	default:
		return 0
	}
}

func (p PoRepProofPartitions) ProofLen() int {
	switch p {
	case TwoPoRepProofPartitions:
		return SinglePartitionProofLen * 2
	default:
		return 0
	}
}

func NewPoRepProofPartitions(partitions int) PoRepProofPartitions {
	switch partitions {
	case 2:
		return TwoPoRepProofPartitions
	default:
		return UnknownPoRepProofPartitions
	}
}
