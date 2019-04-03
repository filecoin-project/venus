package proofs

import (
	"bytes"
	"sort"
)

// VerifySealRequest represents a request to verify the output of a Seal() operation.
type VerifySealRequest struct {
	CommD      CommD     // returned from seal
	CommR      CommR     // returned from seal
	CommRStar  CommRStar // returned from seal
	Proof      SealProof // returned from Seal
	ProverID   [31]byte  // uniquely identifies miner
	SectorID   [31]byte  // uniquely identifies sector
	ProofsMode Mode      // used to control sealing/verification performance
}

// VerifyPoSTRequest represents a request to generate verify a proof-of-spacetime.
type VerifyPoSTRequest struct {
	ChallengeSeed PoStChallengeSeed
	SortedCommRs  SortedCommRs
	Faults        []uint64
	Proofs        []PoStProof
	ProofsMode    Mode // used to control sealing/verification performance
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

// Mode configures sealing, sector packing, PoSt generation and other behaviors
// of libfilecoin_proofs. Use Test mode to seal and generate PoSts quickly over
// tiny sectors. Use Live when operating a real Filecoin node.
type Mode int

const (
	// TestMode changes sealing, sector packing, PoSt, etc. to be compatible with test environments
	TestMode = Mode(iota)
	// LiveMode changes sealing, sector packing, PoSt, etc. to be compatible with non-test environments
	LiveMode
)

// SortedCommRs is a slice of CommRs that has deterministic ordering.
type SortedCommRs struct {
	c []CommR
}

// NewSortedCommRs returns a SortedCommRs with the given CommRs
func NewSortedCommRs(commRs ...CommR) SortedCommRs {
	fn := func(i, j int) bool {
		return bytes.Compare(commRs[i][:], commRs[j][:]) == -1
	}

	sort.Slice(commRs[:], fn)

	return SortedCommRs{
		c: commRs,
	}
}

// Values returns the sorted CommRs as a slice
func (s *SortedCommRs) Values() []CommR {
	return s.c
}
