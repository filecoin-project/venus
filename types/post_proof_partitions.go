package types

import (
	"fmt"
	"github.com/pkg/errors"
)

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
		panic(fmt.Sprintf("unexpected value %v", p))
	}
}

// ProofLen returns an integer representing the number of bytes in a PoSt proof
// created with this number of partitions.
func (p PoStProofPartitions) ProofLen() int {
	switch p {
	case OnePoStProofPartition:
		return SinglePartitionProofLen
	default:
		panic(fmt.Sprintf("unexpected value %v", p))
	}
}

// NewPoStProofPartitions produces the PoStProofPartitions corresponding to the
// provided integer.
func NewPoStProofPartitions(numPartitions int) (PoStProofPartitions, error) {
	switch numPartitions {
	case 1:
		return OnePoStProofPartition, nil
	default:
		return UnknownPoStProofPartitions, errors.Errorf("unexpected value %v", numPartitions)
	}
}
