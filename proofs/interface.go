package proofs

// VerifySealRequest represents a request to verify the output of a Seal() operation.
type VerifySealRequest struct {
	CommD     CommD           // returned from seal
	CommR     CommR           // returned from seal
	CommRStar CommRStar       // returned from seal
	Proof     SealProof       // returned from Seal
	ProverID  [31]byte        // uniquely identifies miner
	SectorID  [31]byte        // uniquely identifies sector
	StoreType SectorStoreType // used to control sealing/verification performance
}

// VerifyPoSTRequest represents a request to generate verify a proof-of-spacetime.
type VerifyPoSTRequest struct {
	ChallengeSeed PoStChallengeSeed
	CommRs        []CommR
	Faults        []uint64
	Proofs        []PoStProof
	StoreType     SectorStoreType // used to control sealing/verification performance
}

// VerifyPoSTResponse communicates the validity of a provided proof-of-spacetime.
type VerifyPoSTResponse struct {
	IsValid bool
}

// VerifySealResponse communicates the validity of a provided proof-of-replication.
type VerifySealResponse struct {
	IsValid bool
}

// Verifier provides an interface to the proving subsystem.
type Verifier interface {
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
)
