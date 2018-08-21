package proofs

import (
	"unsafe"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
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
func (rp *RustProver) Seal(req SealRequest) (res SealResponse, err error) {
	// passing arrays in C (lengths are hard-coded on Rust side)
	proverIDPtr := (*C.uchar)(unsafe.Pointer(&req.ProverID[0]))
	challengeSeedPtr := (*C.uchar)(unsafe.Pointer(&req.ChallengeSeed[0]))
	randomSeedPtr := (*C.uchar)(unsafe.Pointer(&req.RandomSeed[0]))

	// the seal function will write into this array; prevents Go from having
	// to make a second call into Rust to deallocate
	out := make([]uint8, 64)
	outPtr := (*C.uchar)(unsafe.Pointer(&out[0]))

	unsealed := C.CString(req.UnsealedPath)
	defer C.free(unsafe.Pointer(unsealed))

	sealed := C.CString(req.SealedPath)
	defer C.free(unsafe.Pointer(sealed))

	// mutates the out-array
	code := C.seal(unsealed, sealed, proverIDPtr, challengeSeedPtr, randomSeedPtr, outPtr)

	if code != 0 {
		err = errors.New(errorString(code))
	} else {
		res = SealResponse{
			Commitments: CommitmentPair{
				CommR: out[0:32],
				CommD: out[32:64],
			},
		}
	}

	return
}

// VerifySeal returns nil if the Seal operation from which its inputs were
// derived was valid, and an error if not.
func (rp *RustProver) VerifySeal(req VerifySealRequest) error {
	commRPtr := (*C.uchar)(unsafe.Pointer(&(req.Commitments.CommR)[0]))
	commDPtr := (*C.uchar)(unsafe.Pointer(&(req.Commitments.CommD)[0]))

	code := C.verify_seal(commRPtr, commDPtr)

	if code != 0 {
		return errors.New(errorString(code))
	}

	return nil
}

func errorString(code C.uint8_t) string {
	status := C.status_to_string(code)
	defer C.free(unsafe.Pointer(status))

	return C.GoString(status)
}
