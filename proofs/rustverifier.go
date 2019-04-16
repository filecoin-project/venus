package proofs

import (
	"time"
	"unsafe"

	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/types"
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

	size, err := cFFISectorSize(req.SectorSize)
	if err != nil {
		return VerifySealResponse{}, err
	}

	// a mutable pointer to a VerifySealResponse C-struct
	resPtr := (*C.VerifySealResponse)(unsafe.Pointer(C.verify_seal(
		size,
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
	commRs := req.SortedCommRs.Values()
	flattened := make([]byte, 32*len(commRs))
	for idx, commR := range commRs {
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

	size, err := cFFISectorSize(req.SectorSize)
	if err != nil {
		return VerifyPoSTResponse{}, err
	}

	// a mutable pointer to a VerifyPoSTResponse C-struct
	resPtr := (*C.VerifyPoSTResponse)(unsafe.Pointer(C.verify_post(
		size,
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

// GetMaxUserBytesPerStagedSector returns the number of user bytes that will fit
// into a staged sector. Due to bit-padding, the number of user bytes that will
// fit into the staged sector will be less than number of bytes reported in
// types.SectorSize.
func GetMaxUserBytesPerStagedSector(size types.SectorSize) (uint64, error) {
	fsize, err := cFFISectorSize(size)
	if err != nil {
		return 0, errors.Wrap(err, "CSectorStoreType failed")
	}

	return uint64(C.get_max_user_bytes_per_staged_sector(fsize)), nil
}

// cPoStProofs copies bytes from the provided PoSt proofs to a C array and
// returns a pointer to that array and its size. Callers are responsible for
// freeing the pointer. If they do not do that, the array will be leaked.
func cPoStProofs(src []types.PoStProof) (*C.uint8_t, C.size_t) {
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

// cFFISectorSize marshals to the FFI type C.FFISectorSize.
func cFFISectorSize(ss types.SectorSize) (C.FFISectorSize, error) {
	switch ss {
	case types.OneKiBSectorSize:
		return C.FFISectorSize(C.SSB_OneKiB), nil
	case types.TwoHundredFiftySixMiBSectorSize:
		return C.FFISectorSize(C.SSB_TwoHundredFiftySixMiB), nil
	default:
		return C.FFISectorSize(0), errors.Errorf("unhandled value: %v", ss)
	}
}
