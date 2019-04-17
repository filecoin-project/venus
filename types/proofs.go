package types

// SinglePartitionProofLen represents the number of bytes in a single partition
// PoRep or PoSt proof. The total length of a PoSt or PoRep proof equals the
// product of SinglePartitionProofLen and the number of partitions.
const SinglePartitionProofLen int = 192

// PoStProof is the byte representation of the Proof of SpaceTime proof
type PoStProof []byte

// PoRepProof is the byte representation of the Seal Proof of Replication
type PoRepProof []byte

// ProofPartitions returns the number of partitions used to create the PoRep
// proof.
func (s PoRepProof) ProofPartitions() PoRepProofPartitions {
	return NewPoRepProofPartitions(len(s) / SinglePartitionProofLen)
}

// ProofPartitions returns the number of partitions used to create the PoSt
// proof.
func (s PoStProof) ProofPartitions() PoStProofPartitions {
	return NewPoStProofPartitions(len(s) / SinglePartitionProofLen)
}
