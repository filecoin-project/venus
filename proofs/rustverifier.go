package proofs

import (
	"time"
	"unsafe"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	logging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log"
)

// #cgo LDFLAGS: -L${SRCDIR}/lib -lfilecoin_proofs
// #cgo pkg-config: ${SRCDIR}/lib/pkgconfig/libfilecoin_proofs.pc
// #include "./include/libfilecoin_proofs.h"
import "C"

var log = logging.Logger("fps") // nolint: deadcode

// RustVerifier provides proof-verification methods.
type RustVerifier struct{}

var _ Verifier = &RustVerifier{}

func elapsed(what string) func() {
	start := time.Now()
	return func() {
		log.Debugf("%s took %v\n", what, time.Since(start))
	}
}

// VerifySeal returns nil if the Seal operation from which its inputs were
// derived was valid, and an error if not.
func (rp *RustVerifier) VerifySeal(req VerifySealRequest) (VerifySealResponse, error) {
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

// VerifyPoST verifies that a proof-of-spacetime is valid.
func (rp *RustVerifier) VerifyPoST(req VerifyPoSTRequest) (VerifyPoSTResponse, error) {
	defer elapsed("VerifyPoST")()

	// flattening the byte slice makes it easier to copy into the C heap
	flattened := make([]byte, 32*len(req.CommRs))
	for idx, commR := range req.CommRs {
		copy(flattened[(32*idx):(32*(1+idx))], commR[:])
	}

	// copy bytes from Go to C heap
	flattenedCommRsCBytes := C.CBytes(flattened)
	defer C.free(flattenedCommRsCBytes)

	challengeSeedCBytes := C.CBytes(req.ChallengeSeed[:])
	defer C.free(challengeSeedCBytes)

	proofsPtr, proofsLen := cPoStProofs(req.Proofs)
	defer C.free(unsafe.Pointer(proofsPtr))

	// allocate fixed-length array of uint64s in C heap
	faultsPtr, faultsSize := cUint64s(req.Faults)
	defer C.free(unsafe.Pointer(faultsPtr))

	cfg, err := CSectorStoreType(req.StoreType)
	if err != nil {
		return VerifyPoSTResponse{}, err
	}

	// a mutable pointer to a VerifyPoSTResponse C-struct
	resPtr := (*C.VerifyPoSTResponse)(unsafe.Pointer(C.verify_post(
		cfg,
		(*C.uint8_t)(flattenedCommRsCBytes),
		C.size_t(len(flattened)),
		(*[32]C.uint8_t)(challengeSeedCBytes),
		proofsPtr,
		proofsLen,
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

// cPoStProofs copies bytes from the provided PoSt proofs to a C array and
// returns a pointer to that array and its size. Callers are responsible for
// freeing the pointer. If they do not do that, the array will be leaked.
func cPoStProofs(src []PoStProof) (*C.uint8_t, C.size_t) {
	flattenedLen := C.size_t(192 * len(src))

	// flattening the byte slice makes it easier to copy into the C heap
	flattened := make([]byte, flattenedLen)
	for idx, proof := range src {
		copy(flattened[(192*idx):(192*(1+idx))], proof[:])
	}

	return (*C.uint8_t)(C.CBytes(flattened)), flattenedLen
}

// cUint64s copies the contents of a slice into a C heap-allocated array and
// returns a pointer to that array and its size. Callers are responsible for
// freeing the pointer. If they do not do that, the array will be leaked.
func cUint64s(src []uint64) (*C.uint64_t, C.size_t) {
	srcCSizeT := C.size_t(len(src))

	// allocate array in C heap
	cUint64s := C.malloc(srcCSizeT * C.sizeof_uint64_t)

	// create a Go slice backed by the C-array
	pp := (*[1 << 30]C.uint64_t)(cUint64s)
	for i, v := range src {
		pp[i] = C.uint64_t(v)
	}

	return (*C.uint64_t)(cUint64s), srcCSizeT
}

// CSectorStoreType marshals from SectorStoreType to the FFI type
// *C.ConfiguredStore.
func CSectorStoreType(cfg SectorStoreType) (*C.ConfiguredStore, error) {
	var scfg C.ConfiguredStore
	if cfg == Live {
		scfg = C.ConfiguredStore(C.Live)
	} else if cfg == Test {
		scfg = C.ConfiguredStore(C.Test)
	} else {
		return nil, errors.Errorf("unknown sector store type: %v", cfg)
	}

	return &scfg, nil
}
