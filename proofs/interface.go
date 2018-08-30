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
	CommD      []byte // data commitment: 32-byte merkle root of raw data
	CommR      []byte // replica commitment: 32-byte merkle root of replicated data
	SnarkProof []byte // proof
}

// UnsealResponse contains contains the number of bytes unsealed (and written) by Unseal().
type UnsealResponse struct {
	NumBytesWritten uint64
}

// UnsealRequest represents a request to unseal a sector.
type UnsealRequest struct {
	NumBytes    uint64 // number of bytes to unseal (corresponds to contents of unsealed sector-file)
	OutputPath  string // path to write unsealed file-bytes
	ProverID    []byte // len=31, uniquely identifies miner
	SealedPath  string // path to sealed sector-file
	StartOffset uint64 // zero-based byte offset in original, unsealed sector-file
}

// VerifySealRequest represents a request to verify the output of a Seal() operation.
type VerifySealRequest struct {
	ChallengeSeed []byte // len=32, pseudo-random data derived from chain
	CommD         []byte // returned from seal
	CommR         []byte // returned from seal
	ProverID      []byte // len=31, uniquely identifies miner
	SnarkProof    []byte // returned from Seal
}

// Prover provides an interface to the proving subsystem.
type Prover interface {
	Seal(SealRequest) (SealResponse, error)
	Unseal(UnsealRequest) (UnsealResponse, error)
	VerifySeal(VerifySealRequest) error
}

// SectorStore provides a mechanism for dispensing sector access
type SectorStore interface {
	NewSealedSectorAccess() (string, error)
	NewStagingSectorAccess() (string, error)
}
