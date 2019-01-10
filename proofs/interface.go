package proofs

// VerifySealRequest represents a request to verify the output of a Seal() operation.
type VerifySealRequest struct {
	CommD     [32]byte        // returned from seal
	CommR     [32]byte        // returned from seal
	CommRStar [32]byte        // returned from seal
	Proof     SealProof       // returned from Seal
	ProverID  [31]byte        // uniquely identifies miner
	SectorID  [31]byte        // uniquely identifies sector
	StoreType SectorStoreType // used to control sealing/verification performance
}

// GeneratePoSTRequest represents a request to generate a proof-of-spacetime.
type GeneratePoSTRequest struct {
	CommRs        [][32]byte
	ChallengeSeed PoStChallengeSeed
}

// VerifyPoSTRequest represents a request to generate verify a proof-of-spacetime.
type VerifyPoSTRequest struct {
	ChallengeSeed PoStChallengeSeed
	CommRs        [][32]byte
	Faults        []uint64
	Proof         PoStProof
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
	Faults []uint64
	Proof  PoStProof
}

// Verifier provides an interface to the proving subsystem.
type Verifier interface {
	GeneratePoST(GeneratePoSTRequest) (GeneratePoSTResponse, error)
	VerifyPoST(VerifyPoSTRequest) (VerifyPoSTResponse, error)
	VerifySeal(VerifySealRequest) (VerifySealResponse, error)
}

// SectorStoreType configures the behavior of the SectorStore used by the SectorBuilder.
type SectorStoreType int

const (
	// Live configures the SectorBuilder to be used by someone operating a real
	// Filecoin node.
	Live = SectorStoreType(iota)
	// Test configures the SectorBuilder to be used with large sectors, in tests.
	Test
	// ProofTest configures the SectorBuilder to perform real proofs against small
	// sectors.
	ProofTest
)
