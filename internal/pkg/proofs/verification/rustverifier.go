package verification

import (
	ffi "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/specs-actors/actors/abi"
)

// FFIBackedProofVerifier calls into rust-fil-proofs through CGO/FFI in order
// to verify PoSt and PoRep proofs
type FFIBackedProofVerifier struct{}

// VerifySeal returns a value indicating the validity of the provided proof
func (f FFIBackedProofVerifier) VerifySeal(info abi.SealVerifyInfo) (bool, error) {
	return ffi.VerifySeal(info)
}

// VerifyPoSt returns a value indicating the validity of the provided proof
func (f FFIBackedProofVerifier) VerifyPoSt(info abi.PoStVerifyInfo) (bool, error) {
	return ffi.VerifyPoSt(info)
}

// NewFFIBackedProofVerifier produces an FFIBackedProofVerifier which delegates
// its verification calls to libfilecoin
func NewFFIBackedProofVerifier() FFIBackedProofVerifier {
	return FFIBackedProofVerifier{}
}
