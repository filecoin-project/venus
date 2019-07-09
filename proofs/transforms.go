package proofs

import (
	"github.com/filecoin-project/go-filecoin/types"
)

// #cgo LDFLAGS: -L${SRCDIR}/lib -lsector_builder_ffi
// #cgo pkg-config: ${SRCDIR}/lib/pkgconfig/sector_builder_ffi.pc
// #include "./include/sector_builder_ffi.h"
import "C"

func cPoStProofs(src []types.PoStProof) (C.uint8_t, *C.uint8_t, C.size_t) {
	proofSize := len(src[0])

	flattenedLen := C.size_t(proofSize * len(src))

	// flattening the byte slice makes it easier to copy into the C heap
	flattened := make([]byte, flattenedLen)
	for idx, proof := range src {
		copy(flattened[(proofSize*idx):(proofSize*(1+idx))], proof[:])
	}

	proofPartitions := proofSize / types.OnePoStProofPartition.ProofLen()

	return C.uint8_t(proofPartitions), (*C.uint8_t)(C.CBytes(flattened)), flattenedLen
}

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

func cSectorClass(c types.SectorClass) (C.sector_builder_ffi_FFISectorClass, error) {
	return C.sector_builder_ffi_FFISectorClass{
		sector_size:            C.uint64_t(c.SectorSize().Uint64()),
		porep_proof_partitions: C.uint8_t(c.PoRepProofPartitions().Int()),
		post_proof_partitions:  C.uint8_t(c.PoStProofPartitions().Int()),
	}, nil
}
