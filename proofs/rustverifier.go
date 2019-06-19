package proofs

import (
	"time"
	"unsafe"

	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/types"
)

// #cgo LDFLAGS: -L${SRCDIR}/lib -lsector_builder_ffi
// #cgo pkg-config: ${SRCDIR}/lib/pkgconfig/sector_builder_ffi.pc
// #include "./include/sector_builder_ffi.h"
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

	// a mutable pointer to a VerifySealResponse C-struct
	resPtr := (*C.sector_builder_ffi_VerifySealResponse)(unsafe.Pointer(C.sector_builder_ffi_verify_seal(
		C.uint64_t(req.SectorSize.Uint64()),
		(*[32]C.uint8_t)(commRCBytes),
		(*[32]C.uint8_t)(commDCBytes),
		(*[32]C.uint8_t)(commRStarCBytes),
		(*[31]C.uint8_t)(proverIDCBytes),
		(*[31]C.uint8_t)(sectorIDCbytes),
		(*C.uint8_t)(proofCBytes),
		C.size_t(len(req.Proof)),
	)))
	defer C.sector_builder_ffi_destroy_verify_seal_response(resPtr)

	if resPtr.status_code != 0 {
		return VerifySealResponse{}, errors.New(C.GoString(resPtr.error_msg))
	}

	return VerifySealResponse{
		IsValid: bool(resPtr.is_valid),
	}, nil
}

// VerifyPoST verifies that a proof-of-spacetime is valid.
func (rp *RustVerifier) VerifyPoST(req VerifyPoStRequest) (VerifyPoSTResponse, error) {
	defer elapsed("VerifyPoST")()

	// validate verification request
	if len(req.Proofs) == 0 {
		return VerifyPoSTResponse{}, errors.New("must provide at least one proof to verify")
	}

	// CommRs must be provided to C.verify_post in the same order that they were
	// provided to the C.generate_post
	commRs := req.SortedCommRs.Values()

	// flattening the byte slice makes it easier to copy into the C heap
	flattened := make([]byte, 32*len(commRs))
	for idx, commR := range commRs {
		copy(flattened[(32*idx):(32*(1+idx))], commR[:])
	}

	// copy bytes from Go to C heap
	flattenedCommRsCBytes := C.CBytes(flattened)
	defer C.free(flattenedCommRsCBytes)

	challengeSeedCBytes := C.CBytes(req.ChallengeSeed[:])
	defer C.free(challengeSeedCBytes)

	proofPartitions, proofsPtr, proofsLen := cPoStProofs(req.Proofs)
	defer C.free(unsafe.Pointer(proofsPtr))

	// allocate fixed-length array of uint64s in C heap
	faultsPtr, faultsSize := cUint64s(req.Faults)
	defer C.free(unsafe.Pointer(faultsPtr))

	// a mutable pointer to a VerifyPoStResponse C-struct
	resPtr := (*C.sector_builder_ffi_VerifyPoStResponse)(unsafe.Pointer(C.sector_builder_ffi_verify_post(
		C.uint64_t(req.SectorSize.Uint64()),
		proofPartitions,
		(*C.uint8_t)(flattenedCommRsCBytes),
		C.size_t(len(flattened)),
		(*[32]C.uint8_t)(challengeSeedCBytes),
		proofsPtr,
		proofsLen,
		faultsPtr,
		faultsSize,
	)))
	defer C.sector_builder_ffi_destroy_verify_post_response(resPtr)

	if resPtr.status_code != 0 {
		return VerifyPoSTResponse{}, errors.New(C.GoString(resPtr.error_msg))
	}

	return VerifyPoSTResponse{
		IsValid: bool(resPtr.is_valid),
	}, nil
}

// GetMaxUserBytesPerStagedSector returns the number of user bytes that will fit
// into a staged sector. Due to bit-padding, the number of user bytes that will
// fit into the staged sector will be less than number of bytes in sectorSize.
func GetMaxUserBytesPerStagedSector(sectorSize *types.BytesAmount) *types.BytesAmount {
	return types.NewBytesAmount(uint64(C.sector_builder_ffi_get_max_user_bytes_per_staged_sector(C.uint64_t(sectorSize.Uint64()))))
}
