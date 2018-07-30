package proofs

// SealRequest represents a request to seal a sector.
type SealRequest struct {
	challengeSeed []uint32 // len=8, pseudo-random data derived from chain
	proverID      []uint8  // len=31, uniquely identifies miner
	randomSeed    []uint32 // len=8, affects DRG topology
}

// SealResponse contains the commitments resulting from a successful Seal().
type SealResponse struct {
	commitments CommitmentPair
}

// CommitmentPair contains commD and commR from the Seal() operation.
type CommitmentPair struct {
	commR uint32 // hard-coded return value (from FPS)
	commD uint32 // hard-coded return value (from FPS)
}

// VerifySealRequest represents a request to verify the output of a Seal() operation.
type VerifySealRequest struct {
	commitments CommitmentPair // returned from Seal
}

// Prover provides an interface to the proving subsystem.
type Prover interface {
	Seal(SealRequest) SealResponse
	VerifySeal(VerifySealRequest) bool
}
