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

var log = logging.Logger("libsectorbuilder") // nolint: deadcode

func elapsed(what string) func() {
	start := time.Now()
	return func() {
		log.Debugf("%s took %v\n", what, time.Since(start))
	}
}

// VerifySeal returns true if the sealing operation from which its inputs were
// derived was valid, and false if not.
func VerifySeal(
	sectorSize *types.BytesAmount,
	commR types.CommR,
	commD types.CommD,
	commRStar types.CommRStar,
	proverId [31]byte,
	sectorId [31]byte,
	proof types.PoRepProof,
) (bool, error) {
	defer elapsed("VerifySeal")()

	commDCBytes := C.CBytes(commD[:])
	defer C.free(commDCBytes)

	commRCBytes := C.CBytes(commR[:])
	defer C.free(commRCBytes)

	commRStarCBytes := C.CBytes(commRStar[:])
	defer C.free(commRStarCBytes)

	proofCBytes := C.CBytes(proof[:])
	defer C.free(proofCBytes)

	proverIDCBytes := C.CBytes(proverId[:])
	defer C.free(proverIDCBytes)

	sectorIDCbytes := C.CBytes(sectorId[:])
	defer C.free(sectorIDCbytes)

	// a mutable pointer to a VerifySealResponse C-struct
	resPtr := (*C.sector_builder_ffi_VerifySealResponse)(unsafe.Pointer(C.sector_builder_ffi_verify_seal(
		C.uint64_t(sectorSize.Uint64()),
		(*[32]C.uint8_t)(commRCBytes),
		(*[32]C.uint8_t)(commDCBytes),
		(*[32]C.uint8_t)(commRStarCBytes),
		(*[31]C.uint8_t)(proverIDCBytes),
		(*[31]C.uint8_t)(sectorIDCbytes),
		(*C.uint8_t)(proofCBytes),
		C.size_t(len(proof)),
	)))
	defer C.sector_builder_ffi_destroy_verify_seal_response(resPtr)

	if resPtr.status_code != 0 {
		return false, errors.New(C.GoString(resPtr.error_msg))
	}

	return bool(resPtr.is_valid), nil
}

// VerifyPoSt returns true if the PoSt-generation operation from which its
// inputs were derived was valid, and false if not.
func VerifyPoSt(
	sectorSize *types.BytesAmount,
	sortedCommRs SortedCommRs,
	challengeSeed types.PoStChallengeSeed,
	proofs []types.PoStProof,
	faults []uint64,
) (bool, error) {
	defer elapsed("VerifyPoSt")()

	// validate verification request
	if len(proofs) == 0 {
		return false, errors.New("must provide at least one proof to verify")
	}

	// CommRs must be provided to C.verify_post in the same order that they were
	// provided to the C.generate_post
	commRs := sortedCommRs.Values()

	// flattening the byte slice makes it easier to copy into the C heap
	flattened := make([]byte, 32*len(commRs))
	for idx, commR := range commRs {
		copy(flattened[(32*idx):(32*(1+idx))], commR[:])
	}

	// copy bytes from Go to C heap
	flattenedCommRsCBytes := C.CBytes(flattened)
	defer C.free(flattenedCommRsCBytes)

	challengeSeedCBytes := C.CBytes(challengeSeed[:])
	defer C.free(challengeSeedCBytes)

	proofPartitions, proofsPtr, proofsLen := cPoStProofs(proofs)
	defer C.free(unsafe.Pointer(proofsPtr))

	// allocate fixed-length array of uint64s in C heap
	faultsPtr, faultsSize := cUint64s(faults)
	defer C.free(unsafe.Pointer(faultsPtr))

	// a mutable pointer to a VerifyPoStResponse C-struct
	resPtr := (*C.sector_builder_ffi_VerifyPoStResponse)(unsafe.Pointer(C.sector_builder_ffi_verify_post(
		C.uint64_t(sectorSize.Uint64()),
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
		return false, errors.New(C.GoString(resPtr.error_msg))
	}

	return bool(resPtr.is_valid), nil
}

// GetMaxUserBytesPerStagedSector returns the number of user bytes that will fit
// into a staged sector. Due to bit-padding, the number of user bytes that will
// fit into the staged sector will be less than number of bytes in sectorSize.
func GetMaxUserBytesPerStagedSector(sectorSize *types.BytesAmount) *types.BytesAmount {
	return types.NewBytesAmount(uint64(C.sector_builder_ffi_get_max_user_bytes_per_staged_sector(C.uint64_t(sectorSize.Uint64()))))
}

// InitSectorBuilder allocates and returns a pointer to a sector builder.
func InitSectorBuilder(
	sectorClass types.SectorClass,
	lastUsedSectorId uint64,
	metadataDir string,
	proverId [31]byte,
	sealedSectorDir string,
	stagedSectorDir string,
	maxNumOpenStagedSectors uint8,
) (unsafe.Pointer, error) {
	defer elapsed("InitSectorBuilder")()

	cMetadataDir := C.CString(metadataDir)
	defer C.free(unsafe.Pointer(cMetadataDir))

	proverIDCBytes := C.CBytes(proverId[:])
	defer C.free(proverIDCBytes)

	cStagedSectorDir := C.CString(stagedSectorDir)
	defer C.free(unsafe.Pointer(cStagedSectorDir))

	cSealedSectorDir := C.CString(sealedSectorDir)
	defer C.free(unsafe.Pointer(cSealedSectorDir))

	class, err := cSectorClass(sectorClass)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get sector class")
	}

	resPtr := (*C.sector_builder_ffi_InitSectorBuilderResponse)(unsafe.Pointer(C.sector_builder_ffi_init_sector_builder(
		class,
		C.uint64_t(lastUsedSectorId),
		cMetadataDir,
		(*[31]C.uint8_t)(proverIDCBytes),
		cSealedSectorDir,
		cStagedSectorDir,
		C.uint8_t(maxNumOpenStagedSectors),
	)))
	defer C.sector_builder_ffi_destroy_init_sector_builder_response(resPtr)

	if resPtr.status_code != 0 {
		return nil, errors.New(C.GoString(resPtr.error_msg))
	}

	return unsafe.Pointer(resPtr.sector_builder), nil
}
