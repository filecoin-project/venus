package proofs

import (
	"unsafe"
)

/*
// explanation of LDFLAGS
//
// -L${SRCDIR}/rust-proofs/target/release       <- Location of the compiled Rust artifacts.
//
// -Wl,xxx                                      <- Pass xxx as an option to the linker. If option contains commas,
//                                                 it is split into multiple options at the commas.
//
// -rpath,${SRCDIR}/rust-proofs/target/release/ <- Location of the runtime library search path (for dynamically-
//                                                 linked libraries).
//
// -lproofs                                     <- Tell the linker to search for libproofs.dylib or libproofs.a in
//                                                 the library search path.
//
#cgo LDFLAGS: -L${SRCDIR}/rust-proofs/target/release -Wl,-rpath,${SRCDIR}/rust-proofs/target/release/ -lproofs
#include "./rust-proofs/libproofs.h"

*/
import "C"

// RustProver provides an interface to rust-proofs.
type RustProver struct{}

var _ Prover = &RustProver{}

// Seal generates and returns a Proof of Replication along with supporting data.
func (rp *RustProver) Seal(req SealRequest) SealResponse {
	// passing arrays in C (lengths are hard-coded on Rust side)
	proverIDPtr := (*C.uchar)(unsafe.Pointer(&req.ProverID[0]))
	challengeSeedPtr := (*C.uchar)(unsafe.Pointer(&req.ChallengeSeed[0]))
	randomSeedPtr := (*C.uchar)(unsafe.Pointer(&req.RandomSeed[0]))

	// the seal function will write into this array; prevents Go from having
	// to make a second call into Rust to deallocate
	out := make([]uint8, 64)
	outPtr := (*C.uchar)(unsafe.Pointer(&out[0]))

	// mutates the out-array
	C.seal(C.CString(req.UnsealedPath), C.CString(req.SealedPath), proverIDPtr, challengeSeedPtr, randomSeedPtr, outPtr)

	return SealResponse{
		Commitments: CommitmentPair{
			CommR: out[0:32],
			CommD: out[32:64],
		},
	}
}

// VerifySeal returns true if the Seal operation from which its inputs were derived was valid.
func (rp *RustProver) VerifySeal(req VerifySealRequest) bool {
	commRPtr := (*C.uchar)(unsafe.Pointer(&(req.Commitments.CommR)[0]))
	commDPtr := (*C.uchar)(unsafe.Pointer(&(req.Commitments.CommD)[0]))

	return (bool)(C.verify_seal(commRPtr, commDPtr))
}
