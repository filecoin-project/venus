package types

import (
	"fmt"
	"github.com/pkg/errors"
)

// PoStProofPartitions represents the number of partitions used when creating a
// PoSt proof, and impacts the size of the proof.
type PoStProofPartitions int

const (
	// UnknownPoStProofPartitions is an opaque value signaling that an unknown number of
	// partitions were used when creating a PoSt proof in test mode.
	UnknownPoStProofPartitions = PoStProofPartitions(iota)

	// OnePoStProofPartition indicates that a single partition was used to create a PoSt proof.
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
