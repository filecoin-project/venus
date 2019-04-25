package sectorbuilder

import (
	"unsafe"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/ipfs/go-cid"
)

// #cgo LDFLAGS: -L${SRCDIR}/../lib -lfilecoin_proofs
// #cgo pkg-config: ${SRCDIR}/../lib/pkgconfig/libfilecoin_proofs.pc
// #include "../include/libfilecoin_proofs.h"
import "C"

func goBytes(src *C.uint8_t, size C.size_t) []byte {
	return C.GoBytes(unsafe.Pointer(src), C.int(size))
}

func goPoStProofs(partitions C.uint8_t, src *C.uint8_t, size C.size_t) ([]types.PoStProof, error) {
	tmp := goBytes(src, size)

	arraySize := len(tmp)
	chunkSize := goPoStProofPartitions(partitions).ProofLen()

	out := make([]types.PoStProof, arraySize/chunkSize)
	for i := 0; i < len(out); i++ {
		out[i] = append(types.PoStProof{}, tmp[i*chunkSize:(i+1)*chunkSize]...)
	}

	return out, nil
}

func goUint64s(src *C.uint64_t, size C.size_t) []uint64 {
	out := make([]uint64, size)
	if src != nil {
		copy(out, (*(*[1 << 30]uint64)(unsafe.Pointer(src)))[:size:size])
	}
	return out
}

func goStagedSectorMetadata(src *C.FFIStagedSectorMetadata, size C.size_t) ([]*stagedSectorMetadata, error) {
	sectors := make([]*stagedSectorMetadata, size)
	if src == nil || size == 0 {
		return sectors, nil
	}

	sectorPtrs := (*[1 << 30]C.FFIStagedSectorMetadata)(unsafe.Pointer(src))[:size:size]
	for i := 0; i < int(size); i++ {
		sectors[i] = &stagedSectorMetadata{
			sectorID: uint64(sectorPtrs[i].sector_id),
		}
	}

	return sectors, nil
}

func goPieceInfos(src *C.FFIPieceMetadata, size C.size_t) ([]*PieceInfo, error) {
	ps := make([]*PieceInfo, size)
	if src == nil || size == 0 {
		return ps, nil
	}

	ptrs := (*[1 << 30]C.FFIPieceMetadata)(unsafe.Pointer(src))[:size:size]
	for i := 0; i < int(size); i++ {
		ref, err := cid.Decode(C.GoString(ptrs[i].piece_key))
		if err != nil {
			return nil, err
		}

		ps[i] = &PieceInfo{
			Ref:  ref,
			Size: uint64(ptrs[i].num_bytes),
		}
	}

	return ps, nil
}

func goPoStProofPartitions(partitions C.uint8_t) types.PoStProofPartitions {
	switch int(partitions) {
	case 1:
		return types.OnePoStProofPartition
	default:
		return types.UnknownPoStProofPartitions
	}
}

func cSectorClass(c types.SectorClass) (C.FFISectorClass, error) {
	return C.FFISectorClass{
		sector_size:            C.uint64_t(c.SectorSize().Uint64()),
		porep_proof_partitions: C.uint8_t(c.PoRepProofPartitions().Uint8()),
		post_proof_partitions:  C.uint8_t(c.PoStProofPartitions().Uint8()),
	}, nil
}
