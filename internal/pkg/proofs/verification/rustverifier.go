package verification

import ffi "github.com/filecoin-project/filecoin-ffi"

// FFIBackedProofVerifier calls into rust-fil-proofs through CGO/FFI in order
// to verify PoSt and PoRep proofs
type FFIBackedProofVerifier struct{}

// VerifySeal returns a value indicating the validity of the provided proof
func (f FFIBackedProofVerifier) VerifySeal(
	sectorSize uint64,
	commR [ffi.CommitmentBytesLen]byte,
	commD [ffi.CommitmentBytesLen]byte,
	proverID [32]byte,
	ticket [32]byte,
	seed [32]byte,
	sectorID uint64,
	proof []byte,
) (bool, error) {
	return ffi.VerifySeal(sectorSize, commR, commD, proverID, ticket, seed, sectorID, proof)
}

// VerifyPoSt returns a value indicating the validity of the provided proof
func (f FFIBackedProofVerifier) VerifyPoSt(
	sectorSize uint64,
	sectorInfo ffi.SortedPublicSectorInfo,
	randomness [32]byte,
	challengeCount uint64,
	proof []byte,
	winners []ffi.Candidate,
	proverID [32]byte,
) (bool, error) {
	return ffi.VerifyPoSt(sectorSize, sectorInfo, randomness, challengeCount, proof, winners, proverID)
}

// NewFFIBackedProofVerifier produces an FFIBackedProofVerifier which delegates
// its verification calls to libfilecoin
func NewFFIBackedProofVerifier() FFIBackedProofVerifier {
	return FFIBackedProofVerifier{}
}
