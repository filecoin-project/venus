package proofs

import (
	"fmt"
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

char *getSectorAccess_cgo(int x); // forward declare the gateway function

*/
import "C"

//export getSectorAccess
func getSectorAccess(x C.int) *C.char { // nolint: deadcode
	return C.CString(fmt.Sprintf("stub%d", x))
}

// RustProver provides an interface to rust-proofs.
type RustProver struct{}

var _ Prover = &RustProver{}

// Seal generates and returns a Proof of Replication along with supporting data.
func (rp *RustProver) Seal(req SealRequest) SealResponse {
	// passing function pointers through FFI involves a gateway function
	unsealed := (C.SectorAccessor)(unsafe.Pointer(C.getSectorAccess_cgo)) // nolint: unconvert
	sealed := (C.SectorAccessor)(unsafe.Pointer(C.getSectorAccess_cgo))   // nolint: unconvert

	// passing arrays in C (lengths are hard-coded on Rust side)
	proverIDPtr := (*C.uchar)(unsafe.Pointer(&req.proverID[0]))
	challengeSeedPtr := (*C.uint)(unsafe.Pointer(&req.challengeSeed[0]))
	randomSeedPtr := (*C.uint)(unsafe.Pointer(&req.randomSeed[0]))

	// the seal function will write into this array; prevents Go from having
	// to make a second call into Rust to deallocate
	out := make([]uint32, 2)
	outPtr := (*C.uint)(unsafe.Pointer(&out[0]))

	// mutates the out-array
	C.seal(C.int(-1), unsealed, sealed, proverIDPtr, challengeSeedPtr, randomSeedPtr, outPtr)

	return SealResponse{
		commitments: CommitmentPair{
			commR: out[0],
			commD: out[1],
		},
	}
}

// VerifySeal returns true if the Seal operation from which its inputs were derived was valid.
func (rp *RustProver) VerifySeal(req VerifySealRequest) bool {
	return (bool)(C.verifySeal(C.uint(req.commitments.commR), C.uint(req.commitments.commD)))
}
