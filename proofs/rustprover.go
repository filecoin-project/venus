package proofs

import (
	"time"
	"unsafe"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	logging "gx/ipfs/QmcuXC5cxs79ro2cUuHs4HQ2bkDLJUYokwL8aivcX6HW3C/go-log"
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
#cgo LDFLAGS: -L${SRCDIR}/rust-proofs/target/release -Wl,-rpath,\$ORIGIN/lib:${SRCDIR}/rust-proofs/target/release/ -lfilecoin_proofs
#include "./rust-proofs/filecoin-proofs/libfilecoin_proofs.h"
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

	cfg, err := CSectorStoreType(req.StoreType)
	if err != nil {
		return VerifySealResponse{}, err
	}

	// a mutable pointer to a VerifySealResponse C-struct
	resPtr := (*C.VerifySealResponse)(unsafe.Pointer(C.verify_seal(
		cfg,
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
	resPtr := (*C.GeneratePoSTResponse)(
		unsafe.Pointer(
			C.generate_post(
				(*C.uint8_t)(cflattened),
				C.size_t(len(flattened)),
				(*[32]C.uint8_t)(unsafe.Pointer(&(req.ChallengeSeed)[0])),
			)))
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

	// TODO: when VerifyPoSTResponse can take the challenge & t_size as params, uncomment all of this
	//byteLen := len(req.Challenge)
	//challengeArray := make([]byte, byteLen)
	//copy(challengeArray[:], req.Challenge[:])
	//
	// copy the the Go challenge slice into C memory
	//challengeCBytes := C.CBytes(challengeArray)
	//defer C.free(challengeCBytes)
	//
	// cast and pass the challenge, proof and size of challenge to C.VerifyPoSTResponse
	//resPtr := (*C.VerifyPoSTResponse)(
	//	unsafe.Pointer(C.verify_post(
	//		proofPtr,
	//		(*C.uint8_t)(challengeCBytes),
	//		C.size_t(byteLen))))

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

// CSectorStoreType marshals from SectorStoreType to the FFI type
// *C.ConfiguredStore.
func CSectorStoreType(cfg SectorStoreType) (*C.ConfiguredStore, error) {
	var scfg C.ConfiguredStore
	if cfg == Live {
		scfg = C.ConfiguredStore(C.Live)
	} else if cfg == Test {
		scfg = C.ConfiguredStore(C.Test)
	} else if cfg == ProofTest {
		scfg = C.ConfiguredStore(C.ProofTest)
	} else {
		return nil, errors.Errorf("unknown sector store type: %v", cfg)
	}

	return &scfg, nil
}
