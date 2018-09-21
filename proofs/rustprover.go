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
#cgo LDFLAGS: -L${SRCDIR}/rust-proofs/target/release -Wl,-rpath,\$ORIGIN/lib:${SRCDIR}/rust-proofs/target/release/ -lfilecoin_proofs -lsector_base
#include "./rust-proofs/filecoin-proofs/libfilecoin_proofs.h"
#include "./rust-proofs/sector-base/libsector_base.h"
*/
import "C"

// RustProver provides an interface to rust-proofs.
type RustProver struct{}

var _ Prover = &RustProver{}

// Seal generates and returns a Proof of Replication along with supporting data.
func (rp *RustProver) Seal(req SealRequest) (res SealResponse, err error) {
	// passing arrays in C (lengths are hard-coded on Rust side)
	proverIDPtr := (*[31]C.uint8_t)(unsafe.Pointer(&req.ProverID[0]))
	sectorIDPtr := (*[31]C.uint8_t)(unsafe.Pointer(&req.SectorID[0]))

	// the seal function will write into these arrays; prevents Go from having
	// to make a second call into Rust to deallocate
	var commR [32]byte
	var commD [32]byte
	var proof [192]byte

	unsealed := C.CString(req.UnsealedPath)
	defer C.free(unsafe.Pointer(unsealed))

	sealed := C.CString(req.SealedPath)
	defer C.free(unsafe.Pointer(sealed))

	commRPtr := (*[32]C.uint8_t)(unsafe.Pointer(&commR[0]))
	commDPtr := (*[32]C.uint8_t)(unsafe.Pointer(&commD[0]))
	proofPtr := (*[192]C.uint8_t)(unsafe.Pointer(&proof[0]))

	// mutates the out-array
	code := C.seal((*C.Box_SectorStore)(req.Storage.GetCPtr()), unsealed, sealed, proverIDPtr, sectorIDPtr, commRPtr, commDPtr, proofPtr)

	if code != 0 {
		err = errors.New(errorString(code))
	} else {

		res = SealResponse{
			CommR: commR,
			CommD: commD,
			Proof: proof, // TODO: consume the exported constant from FPS
		}
	}

	return
}

// VerifySeal returns nil if the Seal operation from which its inputs were
// derived was valid, and an error if not.
func (rp *RustProver) VerifySeal(req VerifySealRequest) error {
	commRPtr := (*[32]C.uint8_t)(unsafe.Pointer(&(req.CommR)[0]))
	commDPtr := (*[32]C.uint8_t)(unsafe.Pointer(&(req.CommD)[0]))
	proofPtr := (*[192]C.uint8_t)(unsafe.Pointer(&(req.Proof)[0]))
	sectorIDPtr := (*[31]C.uint8_t)(unsafe.Pointer(&(req.SectorID)[0]))
	proverIDPtr := (*[31]C.uint8_t)(unsafe.Pointer(&(req.ProverID)[0]))

	code := C.verify_seal((*C.Box_SectorStore)(req.Storage.GetCPtr()), commRPtr, commDPtr, proverIDPtr, sectorIDPtr, proofPtr)

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

	proverIDPtr := (*[31]C.uint8_t)(unsafe.Pointer(&(req.ProverID)[0]))
	sectorIDPtr := (*[31]C.uint8_t)(unsafe.Pointer(&(req.SectorID)[0]))

	code := C.get_unsealed_range((*C.Box_SectorStore)(req.Storage.GetCPtr()), inPath, outPath, C.uint64_t(req.StartOffset), C.uint64_t(req.NumBytes), proverIDPtr, sectorIDPtr, bytesWrittenPtr)
	if code != 0 {
		return UnsealResponse{}, errors.New(errorString(code))
	}

	return UnsealResponse{
		NumBytesWritten: bytesWritten,
	}, nil
}

func errorString(code C.uint32_t) string {
	status := C.status_to_string(code)
	defer C.free(unsafe.Pointer(status))

	return C.GoString(status)
}
