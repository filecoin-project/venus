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
	NewSealedSectorAccess() (NewSectorAccessResponse, error)
	NewStagingSectorAccess() (NewSectorAccessResponse, error)
	WriteUnsealed(WriteUnsealedRequest) (WriteUnsealedResponse, error)
	TruncateUnsealed(TruncateUnsealedRequest) error
	GetNumBytesUnsealed(GetNumBytesUnsealedRequest) (GetNumBytesUnsealedResponse, error)
}

// WriteUnsealedRequest represents a request to write bytes to an unsealed sector.
type WriteUnsealedRequest struct {
	SectorAccess string
	Data         []byte
}

// TruncateUnsealedRequest represents a request to truncate an unsealed sector.
// TODO: This can disappear if we move metadata <--> file sync from Go to Rust.
type TruncateUnsealedRequest struct {
	SectorAccess string
	NumBytes     uint64 // truncate the unsealed sector to NumBytes
}

// NewSectorAccessResponse contains the sector access provisioned by the sector store.
type NewSectorAccessResponse struct {
	SectorAccess string
}

// WriteUnsealedResponse contains the number of bytes unsealed (and written) by Unseal().
type WriteUnsealedResponse struct {
	NumBytesWritten uint64
}

// GetNumBytesUnsealedRequest represents a request to get the number of bytes in an unsealed sector.
type GetNumBytesUnsealedRequest struct {
	SectorAccess string
}

// GetNumBytesUnsealedResponse contains the number of bytes in an unsealed sector.
type GetNumBytesUnsealedResponse struct {
	NumBytes uint64
}
