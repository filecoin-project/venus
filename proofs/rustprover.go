package proofs

import (
	"time"
	"unsafe"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	logging "gx/ipfs/QmZChCsSt8DctjceaL56Eibc29CVQq4dGKRXC5JRZ6Ppae/go-log"
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

var log = logging.Logger("fps") // nolint: deadcode

// RustProver provides an interface to rust-proofs.
type RustProver struct{}

var _ Prover = &RustProver{}

// SnarkBytesLen is the length of the Proof of SpaceTime proof.
const SnarkBytesLen uint = 192

// SealBytesLen is the length of the proof of Seal Proof of Replication.
const SealBytesLen uint = 384

// PoStProof is the byte representation of the Proof of SpaceTime proof
type PoStProof [SnarkBytesLen]byte

// SealProof is the byte representation of the Seal Proof of Replication
type SealProof [SealBytesLen]byte

func elapsed(what string) func() {
	start := time.Now()
	return func() {
		log.Debugf("%s took %v\n", what, time.Since(start))
	}
}

// Seal generates and returns a Proof of Replication along with supporting data.
func (rp *RustProver) Seal(req SealRequest) (res SealResponse, err error) {
	defer elapsed("Seal")()

	unsealed := C.CString(req.UnsealedPath)
	defer C.free(unsafe.Pointer(unsealed))

	sealed := C.CString(req.SealedPath)
	defer C.free(unsafe.Pointer(sealed))

	proverIDCBytes := C.CBytes(req.ProverID[:])
	defer C.free(proverIDCBytes)

	sectorIDCbytes := C.CBytes(req.SectorID[:])
	defer C.free(sectorIDCbytes)

	// a mutable pointer to a SealResponse C-struct
	resPtr := (*C.SealResponse)(unsafe.Pointer(C.seal(
		(*C.Box_SectorStore)(req.Storage.GetCPtr()),
		unsealed,
		sealed,
		(*[31]C.uint8_t)(proverIDCBytes),
		(*[31]C.uint8_t)(sectorIDCbytes))))
	defer C.destroy_seal_response(resPtr)

	if resPtr.status_code != 0 {
		return SealResponse{}, errors.New(C.GoString(resPtr.error_msg))
	}

	commRSlice := C.GoBytes(unsafe.Pointer(&resPtr.comm_r[0]), 32)
	var commR [32]byte
	copy(commR[:], commRSlice)

	commDSlice := C.GoBytes(unsafe.Pointer(&resPtr.comm_d[0]), 32)
	var commD [32]byte
	copy(commD[:], commDSlice)

	commRStarSlice := C.GoBytes(unsafe.Pointer(&resPtr.comm_r_star[0]), 32)
	var commRStar [32]byte
	copy(commRStar[:], commRStarSlice)

	proofSlice := C.GoBytes(unsafe.Pointer(&resPtr.proof[0]), 384)
	var proof SealProof
	copy(proof[:], proofSlice)

	res = SealResponse{
		CommD:     commD,
		CommR:     commR,
		CommRStar: commRStar,
		Proof:     proof,
	}

	return
}

// VerifySeal returns nil if the Seal operation from which its inputs were
// derived was valid, and an error if not.
func (rp *RustProver) VerifySeal(req VerifySealRequest) (VerifySealResponse, error) {
	defer elapsed("VerifySeal")()

	commDCBytes := C.CBytes(req.CommD[:])
	defer C.free(commDCBytes)

	commRCBytes := C.CBytes(req.CommR[:])
	defer C.free(commRCBytes)

	commRStarCBytes := C.CBytes(req.CommRStar[:])
	defer C.free(commRStarCBytes)

	proofCBytes := C.CBytes(req.Proof[:])
	defer C.free(proofCBytes)

	proverIDCBytes := C.CBytes(req.ProverID[:])
	defer C.free(proverIDCBytes)

	sectorIDCbytes := C.CBytes(req.SectorID[:])
	defer C.free(sectorIDCbytes)

	// a mutable pointer to a VerifySealResponse C-struct
	resPtr := (*C.VerifySealResponse)(unsafe.Pointer(C.verify_seal(
		(*C.Box_SectorStore)(req.Storage.GetCPtr()),
		(*[32]C.uint8_t)(commRCBytes),
		(*[32]C.uint8_t)(commDCBytes),
		(*[32]C.uint8_t)(commRStarCBytes),
		(*[31]C.uint8_t)(proverIDCBytes),
		(*[31]C.uint8_t)(sectorIDCbytes),
		(*[384]C.uint8_t)(proofCBytes),
	)))
	defer C.destroy_verify_seal_response(resPtr)

	if resPtr.status_code != 0 {
		return VerifySealResponse{}, errors.New(C.GoString(resPtr.error_msg))
	}

	return VerifySealResponse{
		IsValid: bool(resPtr.is_valid),
	}, nil
}

// Unseal unseals and writes the requested number of bytes (respecting the
// provided offset, which is relative to the unsealed sector-file) to
// req.OutputPath. It is possible that req.NumBytes > res.NumBytesWritten.
// If this happens, callers should truncate the file at req.OutputPath back
// to its pre-unseal() number of bytes.
func (rp *RustProver) Unseal(req UnsealRequest) (UnsealResponse, error) {
	defer elapsed("Unseal")()

	inPath := C.CString(req.SealedPath)
	defer C.free(unsafe.Pointer(inPath))

	outPath := C.CString(req.OutputPath)
	defer C.free(unsafe.Pointer(outPath))

	proverIDCBytes := C.CBytes(req.ProverID[:])
	defer C.free(proverIDCBytes)

	sectorIDCbytes := C.CBytes(req.SectorID[:])
	defer C.free(sectorIDCbytes)

	resPtr := (*C.GetUnsealedRangeResponse)(unsafe.Pointer(C.get_unsealed_range(
		(*C.Box_SectorStore)(req.Storage.GetCPtr()),
		inPath,
		outPath,
		C.uint64_t(req.StartOffset),
		C.uint64_t(req.NumBytes),
		(*[31]C.uint8_t)(proverIDCBytes),
		(*[31]C.uint8_t)(sectorIDCbytes))))
	defer C.destroy_get_unsealed_range_response(resPtr)

	if resPtr.status_code != 0 {
		return UnsealResponse{}, errors.New(C.GoString(resPtr.error_msg))
	}

	return UnsealResponse{
		NumBytesWritten: uint64(resPtr.num_bytes_written),
	}, nil
}

// GeneratePoST produces a proof-of-spacetime for the provided commitment replicas.
func (rp *RustProver) GeneratePoST(req GeneratePoSTRequest) (GeneratePoSTResponse, error) {
	defer elapsed("GeneratePoST")()

	// flattening the byte slice makes it easier to copy into the C heap
	flattened := make([]byte, 32*len(req.CommRs))
	for idx, commR := range req.CommRs {
		copy(flattened[(32*idx):(32*(1+idx))], commR[:])
	}

	// copy the Go byte slice into C memory
	cflattened := C.CBytes(flattened)
	defer C.free(cflattened)

	// a mutable pointer to a GeneratePoSTResponse C-struct
	resPtr := (*C.GeneratePoSTResponse)(unsafe.Pointer(C.generate_post(
		(*C.Box_SectorStore)(nil), // TODO: remove this now-unused parameter from rust-proofs
		(*C.uint8_t)(cflattened),
		C.size_t(len(flattened)),
		(*[32]C.uint8_t)(unsafe.Pointer(&(req.ChallengeSeed)[0])))))
	defer C.destroy_generate_post_response(resPtr)

	if resPtr.status_code != 0 {
		return GeneratePoSTResponse{}, errors.New(C.GoString(resPtr.error_msg))
	}

	// copy proof bytes back to Go from C

	proofSlice := C.GoBytes(unsafe.Pointer(&resPtr.proof[0]), C.int(SnarkBytesLen))
	var proof PoStProof
	copy(proof[:], proofSlice)

	return GeneratePoSTResponse{
		Proof:  proof,
		Faults: goUint64s(resPtr.faults_ptr, resPtr.faults_len),
	}, nil
}

// VerifyPoST verifies that a proof-of-spacetime is valid.
func (rp *RustProver) VerifyPoST(req VerifyPoSTRequest) (VerifyPoSTResponse, error) {
	defer elapsed("VerifyPoST")()

	proofPtr := (*[C.uint(SnarkBytesLen)]C.uint8_t)(unsafe.Pointer(&(req.Proof)[0]))

	// a mutable pointer to a VerifyPoSTResponse C-struct
	resPtr := (*C.VerifyPoSTResponse)(unsafe.Pointer(C.verify_post(proofPtr)))
	defer C.destroy_verify_post_response(resPtr)

	if resPtr.status_code != 0 {
		return VerifyPoSTResponse{}, errors.New(C.GoString(resPtr.error_msg))
	}

	return VerifyPoSTResponse{
		IsValid: bool(resPtr.is_valid),
	}, nil
}

// goUint64s accepts a pointer to a C-allocated uint64 and a size and produces
// a Go-managed slice of uint64. Note that this function copies values into the
// Go heap from C.
func goUint64s(src *C.uint64_t, size C.size_t) []uint64 {
	out := make([]uint64, size)
	if src != nil {
		copy(out, (*(*[1 << 30]uint64)(unsafe.Pointer(src)))[:size:size])
	}
	return out
}
