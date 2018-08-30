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
	out := make([]uint8, 260)
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
			CommR:      out[0:32],
			CommD:      out[32:64],
			SnarkProof: out[64:260], // TODO: consume the exported constant from FPS
		}
	}

	return
}

// VerifySeal returns nil if the Seal operation from which its inputs were
// derived was valid, and an error if not.
func (rp *RustProver) VerifySeal(req VerifySealRequest) error {
	commRPtr := (*C.uchar)(unsafe.Pointer(&(req.CommR)[0]))
	commDPtr := (*C.uchar)(unsafe.Pointer(&(req.CommD)[0]))
	snarkPtr := (*C.uchar)(unsafe.Pointer(&(req.SnarkProof)[0]))
	cSeedPtr := (*C.uchar)(unsafe.Pointer(&(req.ChallengeSeed)[0]))
	provrPtr := (*C.uchar)(unsafe.Pointer(&(req.ProverID)[0]))

	code := C.verify_seal(commRPtr, commDPtr, provrPtr, cSeedPtr, snarkPtr)

	if code != 0 {
		return errors.New(errorString(code))
	}

	return nil
}

// Unseal unseales and writes the requested number of bytes (respecting the
// provided offset, which is relative to the unsealed sector-file) to
// req.OutputPath. It is possible that req.NumBytes > res.NumBytesWritten.
// If this happens, callers should truncate the file at req.OutputPath back
// to its pre-unseal() number of bytes.
func (rp *RustProver) Unseal(req UnsealRequest) (UnsealResponse, error) {
	inPath := C.CString(req.SealedPath)
	defer C.free(unsafe.Pointer(inPath))

	outPath := C.CString(req.OutputPath)
	defer C.free(unsafe.Pointer(outPath))

	// The unseal function will write to bytesWrittenPtr to indicate the number
	// of bytes which have been written to the outPath.
	var bytesWritten uint64
	bytesWrittenPtr := (*C.uint64_t)(unsafe.Pointer(&bytesWritten))

	provrPtr := (*C.uchar)(unsafe.Pointer(&(req.ProverID)[0]))

	code := C.get_unsealed_range(inPath, outPath, C.uint64_t(req.StartOffset), C.uint64_t(req.NumBytes), provrPtr, bytesWrittenPtr)
	if code != 0 {
		return UnsealResponse{}, errors.New(errorString(code))
	}

	return UnsealResponse{
		NumBytesWritten: bytesWritten,
	}, nil
}

func errorString(code C.uint8_t) string {
	status := C.status_to_string(code)
	defer C.free(unsafe.Pointer(status))

	return C.GoString(status)
}
