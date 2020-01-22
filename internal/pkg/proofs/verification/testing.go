package verification

import (
	ffi "github.com/filecoin-project/filecoin-ffi"
)

// VerifySealRequest represents a request to verify the output of a Seal() operation.
type VerifySealRequest struct {
	SectorSize uint64
	CommR      [ffi.CommitmentBytesLen]byte
	CommD      [ffi.CommitmentBytesLen]byte
	ProverID   [32]byte
	Ticket     [32]byte
	Seed       [32]byte
	SectorID   uint64
	Proof      []byte
}

// VerifyPoStRequest represents a request to generate verify a proof-of-spacetime.
type VerifyPoStRequest struct {
	SectorSize     uint64
	SectorInfo     ffi.SortedPublicSectorInfo
	Randomness     [32]byte
	ChallengeCount uint64
	Proof          []byte
	Winners        []ffi.Candidate
	ProverID       [32]byte
}

// FakeVerifier is a simple mock Verifier for testing.
type FakeVerifier struct {
	VerifyPoStValid bool
	VerifyPoStError error
	VerifySealValid bool
	VerifySealError error

	// these requests will be captured by code that calls VerifySeal or VerifyPoSt
	LastReceivedVerifySealRequest *VerifySealRequest
	LastReceivedVerifyPoStRequest *VerifyPoStRequest
}

var _ PoStVerifier = (*FakeVerifier)(nil)
var _ SealVerifier = (*FakeVerifier)(nil)

// VerifySeal fakes out seal (PoRep) proof verification, skipping an FFI call to
// verify_seal.
func (f *FakeVerifier) VerifySeal(
	sectorSize uint64,
	commR [ffi.CommitmentBytesLen]byte,
	commD [ffi.CommitmentBytesLen]byte,
	proverID [32]byte,
	ticket [32]byte,
	seed [32]byte,
	sectorID uint64,
	proof []byte,
) (bool, error) {
	f.LastReceivedVerifySealRequest = &VerifySealRequest{
		SectorSize: sectorSize,
		CommR:      commR,
		CommD:      commD,
		ProverID:   proverID,
		Ticket:     ticket,
		Seed:       seed,
		SectorID:   sectorID,
		Proof:      proof,
	}
	return f.VerifySealValid, f.VerifySealError
}

// VerifyPoSt fakes out PoSt proof verification, skipping an FFI call to
// generate_post.
func (f *FakeVerifier) VerifyPoSt(
	sectorSize uint64,
	sectorInfo ffi.SortedPublicSectorInfo,
	randomness [32]byte,
	challengeCount uint64,
	proof []byte,
	winners []ffi.Candidate,
	proverID [32]byte,
) (bool, error) {
	f.LastReceivedVerifyPoStRequest = &VerifyPoStRequest{
		SectorSize:     sectorSize,
		SectorInfo:     sectorInfo,
		Randomness:     randomness,
		ChallengeCount: challengeCount,
		Proof:          proof,
		Winners:        winners,
		ProverID:       proverID,
	}
	return f.VerifyPoStValid, f.VerifyPoStError
}
