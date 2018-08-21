package proofs

// SealRequest represents a request to seal a sector.
type SealRequest struct {
	UnsealedPath  string // path to unsealed sector-file
	SealedPath    string // path to sealed sector-file
	ChallengeSeed []byte // len=32, pseudo-random data derived from chain
	ProverID      []byte // len=31, uniquely identifies miner
	RandomSeed    []byte // len=32, affects DRG topology
}

// SealResponse contains the commitments resulting from a successful Seal().
type SealResponse struct {
	Commitments CommitmentPair
}

// CommitmentPair contains commD and commR from the Seal() operation.
type CommitmentPair struct {
	CommR []byte // replica commitment: 32-byte merkle root of replicated data
	CommD []byte // data commitment: 32-byte merkle root of raw data
}

// VerifySealRequest represents a request to verify the output of a Seal() operation.
type VerifySealRequest struct {
	Commitments CommitmentPair // returned from Seal
}

// Prover provides an interface to the proving subsystem.
type Prover interface {
	Seal(SealRequest) (SealResponse, error)
	VerifySeal(VerifySealRequest) error
}
