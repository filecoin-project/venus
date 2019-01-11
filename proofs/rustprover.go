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
#cgo darwin LDFLAGS: -L${SRCDIR}/../lib -lfilecoin_proofs -framework Security -lSystem -lresolv -lc -lm
#cgo linux LDFLAGS: -L${SRCDIR}/../lib -lfilecoin_proofs -lutil -lutil -ldl -lrt -lpthread -lgcc_s -lc -lm -lrt -lpthread -lutil -lutil
#include "../lib/libfilecoin_proofs.h"
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

// PoStChallengeSeedBytesLen is the number of bytes in the Proof of SpaceTime challenge seed.
const PoStChallengeSeedBytesLen uint = 32

// PoStProof is the byte representation of the Proof of SpaceTime proof
type PoStProof [SnarkBytesLen]byte

// SealProof is the byte representation of the Seal Proof of Replication
type SealProof [SealBytesLen]byte

// PoStChallengeSeed is an input to the proof-of-spacetime generation and verification methods.
type PoStChallengeSeed [PoStChallengeSeedBytesLen]byte

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

	// flattening the byte slice makes it easier to copy into the C heap
	flattened := make([]byte, 32*len(req.CommRs))
	for idx, commR := range req.CommRs {
		copy(flattened[(32*idx):(32*(1+idx))], commR[:])
	}

	// copy bytes from Go to C heap
	flattedCommRsCBytes := C.CBytes(flattened)
	defer C.free(flattedCommRsCBytes)

	challengeSeedCBytes := C.CBytes(req.ChallengeSeed[:])
	defer C.free(challengeSeedCBytes)

	proofCBytes := C.CBytes(req.Proof[:])
	defer C.free(proofCBytes)

	// allocate fixed-length array of uint64s in C heap
	faultsPtr, faultsSize := cUint64s(req.Faults)
	defer C.free(unsafe.Pointer(faultsPtr))

	// a mutable pointer to a VerifyPoSTResponse C-struct
	resPtr := (*C.VerifyPoSTResponse)(unsafe.Pointer(C.verify_post(
		(*C.uint8_t)(flattedCommRsCBytes),
		C.size_t(len(flattened)),
		(*[32]C.uint8_t)(challengeSeedCBytes),
		(*[192]C.uint8_t)(proofCBytes),
		faultsPtr,
		faultsSize,
	)))
	defer C.destroy_verify_post_response(resPtr)

	if resPtr.status_code != 0 {
		return VerifyPoSTResponse{}, errors.New(C.GoString(resPtr.error_msg))
	}

	return VerifyPoSTResponse{
		// TODO: change this to the bool statement
		// See https://github.com/filecoin-project/go-filecoin/issues/1302
		// bool(resPtr.is_valid),
		IsValid: true,
	}, nil
}

// cUint64s copies the contents of a slice into a C heap-allocated array and
// returns a pointer to that array and its size. Callers are responsible for
// freeing the pointer. If they do not do that, the array will be leaked.
func cUint64s(src []uint64) (*C.uint64_t, C.size_t) {
	srcCSizeT := C.size_t(len(src))

	// allocate array in C heap
	cUint64s := C.malloc(*C.sizeof_uint64_t)

	// create a Go slice backed by the C-array
	pp := (*[1 << 30]C.uint64_t)(cUint64s)
	for i, v := range src {
		pp[i] = C.uint64_t(v)
	}

	return (*C.uint64_t)(cUint64s), srcCSizeT
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
