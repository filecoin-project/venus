package sectorbuilder

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"time"
	"unsafe"

	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
)

/*
#cgo LDFLAGS: -L${SRCDIR}/../proofs/rust-proofs/target/release -Wl,-rpath,\$ORIGIN/lib:${SRCDIR}/../proofs/rust-proofs/target/release/ -lfilecoin_proofs -lsector_base
#include "../proofs/rust-proofs/filecoin-proofs/libfilecoin_proofs.h"
#include "../proofs/rust-proofs/sector-base/libsector_base.h"

*/
import "C"

func elapsed(what string) func() {
	start := time.Now()
	return func() {
		log.Debugf("%s took %v\n", what, time.Since(start))
	}
}

// RustSectorBuilder is a struct which serves as a proxy for a SectorStore in Rust.
type RustSectorBuilder struct {
	ptr unsafe.Pointer
}

var _ SectorBuilder = &RustSectorBuilder{}

// NewRustSectorBuilder instantiates a SectorBuilder through the FFI.
func NewRustSectorBuilder(lastUsedSectorID uint64, metadataDir string, proverID [31]byte, stagedSectorDir string, sealedSectorDir string) *RustSectorBuilder {
	cMetadataDir := C.CString(metadataDir)
	defer C.free(unsafe.Pointer(cMetadataDir))

	proverIDCBytes := C.CBytes(proverID[:])
	defer C.free(proverIDCBytes)

	cStagedSectorDir := C.CString(stagedSectorDir)
	defer C.free(unsafe.Pointer(cStagedSectorDir))

	cSealedSectorDir := C.CString(sealedSectorDir)
	defer C.free(unsafe.Pointer(cSealedSectorDir))

	ptr := C.init_sector_builder(
		C.uint64_t(lastUsedSectorID),
		cMetadataDir,
		(*[31]C.uint8_t)(proverIDCBytes),
		cStagedSectorDir,
		cSealedSectorDir,
	)

	sb := &RustSectorBuilder{
		ptr: unsafe.Pointer(ptr),
	}

	runtime.SetFinalizer(sb, func(o *RustSectorBuilder) {
		o.destroy()
	})

	return sb
}

// AddPiece is a stub.
func (sb *RustSectorBuilder) AddPiece(ctx context.Context, pi *PieceInfo) (sectorID uint64, err error) {
	panic("implement me")
}

// ReadPieceFromSealedSector is a stub.
func (sb *RustSectorBuilder) ReadPieceFromSealedSector(pieceCid *cid.Cid) (io.Reader, error) {
	panic("implement me")
}

// SealAllStagedSectors is a stub.
func (sb *RustSectorBuilder) SealAllStagedSectors(ctx context.Context) error {
	panic("implement me")
}

// SealedSectors is a stub.
func (sb *RustSectorBuilder) SealedSectors() []*SealedSector {
	panic("implement me")
}

// SectorSealResults is a stub.
func (sb *RustSectorBuilder) SectorSealResults() <-chan interface{} {
	panic("implement me")
}

// Close is a stub.
func (sb *RustSectorBuilder) Close() error {
	panic("implement me")
}

// printSectorBuilderState prints various internals of the Rust-managed SectorBuilder to stdout.
func (sb *RustSectorBuilder) printSectorBuilderState() {
	defer elapsed("printSectorBuilderState")()

	// Note: The C string is never freed, so this will leak. I'll delete it in
	// the next SectorBuilder port-related PR. It's just here for debugging.
	fmt.Println(C.GoString(C.debug_state((*C.SectorBuilder)(sb.ptr))))
}

// destroy deallocates and destroys a DiskBackedSectorStore.
func (sb *RustSectorBuilder) destroy() {
	C.destroy_sector_builder((*C.SectorBuilder)(sb.ptr))

	sb.ptr = nil
}
