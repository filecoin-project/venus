package types

import (
	"github.com/pkg/errors"
)

// SinglePartitionProofLen represents the number of bytes in a single partition
// PoRep or PoSt proof. The total length of a PoSt or PoRep proof equals the
// product of SinglePartitionProofLen and the number of partitions.
const SinglePartitionProofLen int = 192

// PoStProof is the byte representation of the Proof of SpaceTime proof
type PoStProof []byte

// PoRepProof is the byte representation of the Seal Proof of Replication
type PoRepProof []byte

// ProofPartitions returns the number of partitions used to create the PoRep
// proof, or an error if the PoRep proof has an unsupported length.
func (s PoRepProof) ProofPartitions() (PoRepProofPartitions, error) {
	n := len(s)

	if n%SinglePartitionProofLen != 0 {
		return UnknownPoRepProofPartitions, errors.Errorf("invalid PoRep proof length %d", n)
	}

	return NewPoRepProofPartitions(n / SinglePartitionProofLen)
}

// ProofPartitions returns the number of partitions used to create the PoSt
// proof, or an error if the PoSt proof has an unsupported length.
func (s PoStProof) ProofPartitions() (PoStProofPartitions, error) {
	n := len(s)

	if n%SinglePartitionProofLen != 0 {
		return UnknownPoStProofPartitions, errors.Errorf("invalid PoSt proof length %d", n)
	}

	return NewPoStProofPartitions(n / SinglePartitionProofLen)
}
