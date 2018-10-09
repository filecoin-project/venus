package proofs

import "unsafe"

// SealRequest represents a request to seal a sector.
type SealRequest struct {
	ProverID     [31]byte    // uniquely identifies miner
	SealedPath   string      // path to sealed sector-file
	SectorID     [31]byte    // uniquely identifies sector
	Storage      SectorStore // used to manipulate sectors
	UnsealedPath string      // path to unsealed sector-file
}

// SealResponse contains the commitments resulting from a successful Seal().
type SealResponse struct {
	CommD [32]byte  // data commitment: merkle root of raw data
	CommR [32]byte  // replica commitment: merkle root of replicated data
	Proof [192]byte // proof
}

// UnsealResponse contains contains the number of bytes unsealed (and written) by Unseal().
type UnsealResponse struct {
	NumBytesWritten uint64
}

// UnsealRequest represents a request to unseal a sector.
type UnsealRequest struct {
	NumBytes    uint64      // number of bytes to unseal (corresponds to contents of unsealed sector-file)
	OutputPath  string      // path to write unsealed file-bytes
	ProverID    [31]byte    // uniquely identifies miner
	SealedPath  string      // path to sealed sector-file
	SectorID    [31]byte    // uniquely identifies sector
	StartOffset uint64      // zero-based byte offset in original, unsealed sector-file
	Storage     SectorStore // used to manipulate sectors
}

// VerifySealRequest represents a request to verify the output of a Seal() operation.
type VerifySealRequest struct {
	CommD    [32]byte    // returned from seal
	CommR    [32]byte    // returned from seal
	Proof    [192]byte   // returned from Seal
	ProverID [31]byte    // uniquely identifies miner
	SectorID [31]byte    // uniquely identifies sector
	Storage  SectorStore // used to manipulate sectors
}

// GeneratePoSTRequest represents a request to generate a proof-of-spacetime.
type GeneratePoSTRequest struct {
	CommRs        [][32]byte
	ChallengeSeed [32]byte
}

// VerifyPoSTRequest represents a request to generate verify a proof-of-spacetime.
type VerifyPoSTRequest struct {
	Storage SectorStore
	Proof   [192]byte
}

// VerifyPoSTResponse communicates the validity of a provided proof-of-spacetime.
type VerifyPoSTResponse struct {
	IsValid bool
}

// VerifySealResponse communicates the validity of a provided proof-of-replication.
type VerifySealResponse struct {
	IsValid bool
}

// GeneratePoSTResponse contains PoST proof and any faults that may have occurred.
type GeneratePoSTResponse struct {
	Faults []uint8
	Proof  [192]byte
}

// Prover provides an interface to the proving subsystem.
type Prover interface {
	GeneratePoST(GeneratePoSTRequest) (GeneratePoSTResponse, error)
	Seal(SealRequest) (SealResponse, error)
	Unseal(UnsealRequest) (UnsealResponse, error)
	VerifyPoST(VerifyPoSTRequest) (VerifyPoSTResponse, error)
	VerifySeal(VerifySealRequest) (VerifySealResponse, error)
}

// SectorStore provides a mechanism for dispensing sector access
type SectorStore interface {
	GetCPtr() unsafe.Pointer
	GetMaxUnsealedBytesPerSector() (GetMaxUnsealedBytesPerSectorResponse, error)
	GetNumBytesUnsealed(GetNumBytesUnsealedRequest) (GetNumBytesUnsealedResponse, error)
	NewSealedSectorAccess() (NewSectorAccessResponse, error)
	NewStagingSectorAccess() (NewSectorAccessResponse, error)
	TruncateUnsealed(TruncateUnsealedRequest) error
	WriteUnsealed(WriteUnsealedRequest) (WriteUnsealedResponse, error)
	ReadUnsealed(ReadUnsealedRequest) (ReadUnsealedResponse, error)
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

// GetMaxUnsealedBytesPerSectorResponse contains the number of bytes that will fit into an unsealed sector.
type GetMaxUnsealedBytesPerSectorResponse struct {
	NumBytes uint64
}

// ReadUnsealedRequest is a request to read unadulterated user piece-bytes from a sector access.
type ReadUnsealedRequest struct {
	SectorAccess string
	StartOffset  uint64
	NumBytes     uint64
}

// ReadUnsealedResponse holds the data read from the unsealed sector access.
type ReadUnsealedResponse struct {
	Data []byte
}
