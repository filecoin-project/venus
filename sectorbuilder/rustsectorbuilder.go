package sectorbuilder

import (
	"context"
	"io"
	"runtime"
	"time"
	"unsafe"

	bserv "gx/ipfs/QmTfTKeBhTLjSjxXQsjkF2b1DfZmYEMnknGE2y2gX57C6v/go-blockservice"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	uio "gx/ipfs/Qmdg2crJzNUF1mLPnLPSCCaDdLDqE4Qrh9QEiDooSYkvuB/go-unixfs/io"
	dag "gx/ipfs/QmeLG6jF1xvEmHca5Vy4q4EdQWp8Xq9S6EPyZrN9wvSRLC/go-merkledag"
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
	blockService bserv.BlockService
	ptr          unsafe.Pointer
}

// RustSectorStoreType configures the behavior of the SectorStore used by the SectorBuilder.
type RustSectorStoreType int

const (
	// Live configures the SectorBuilder to be used by someone operating a real
	// Filecoin node.
	Live = RustSectorStoreType(iota)
	// Test configures the SectorBuilder to be used with large sectors, in tests.
	Test
	// ProofTest configures the SectorBuilder to perform real proofs against small
	// sectors.
	ProofTest
)

var _ SectorBuilder = &RustSectorBuilder{}

// NewRustSectorBuilder instantiates a SectorBuilder through the FFI.
func NewRustSectorBuilder(blockService bserv.BlockService, sectorStoreType RustSectorStoreType, lastUsedSectorID uint64, metadataDir string, proverID [31]byte, stagedSectorDir string, sealedSectorDir string) (*RustSectorBuilder, error) {
	defer elapsed("NewRustSectorBuilder")()

	cMetadataDir := C.CString(metadataDir)
	defer C.free(unsafe.Pointer(cMetadataDir))

	proverIDCBytes := C.CBytes(proverID[:])
	defer C.free(proverIDCBytes)

	cStagedSectorDir := C.CString(stagedSectorDir)
	defer C.free(unsafe.Pointer(cStagedSectorDir))

	cSealedSectorDir := C.CString(sealedSectorDir)
	defer C.free(unsafe.Pointer(cSealedSectorDir))

	var cfg C.ConfiguredStore
	if sectorStoreType == Live {
		cfg = C.ConfiguredStore(C.Live)
	} else if sectorStoreType == Test {
		cfg = C.ConfiguredStore(C.Test)
	} else if sectorStoreType == ProofTest {
		cfg = C.ConfiguredStore(C.ProofTest)
	} else {
		return nil, errors.Errorf("unknown sector store type: %v", sectorStoreType)
	}

	resPtr := (*C.InitSectorBuilderResponse)(unsafe.Pointer(C.init_sector_builder(
		(*C.ConfiguredStore)(unsafe.Pointer(&cfg)),
		C.uint64_t(lastUsedSectorID),
		cMetadataDir,
		(*[31]C.uint8_t)(proverIDCBytes),
		cStagedSectorDir,
		cSealedSectorDir,
	)))
	defer C.destroy_init_sector_builder_response(resPtr)

	if resPtr.status_code != 0 {
		return nil, errors.New(C.GoString(resPtr.error_msg))
	}

	sb := &RustSectorBuilder{
		ptr:          unsafe.Pointer(resPtr.sector_builder),
		blockService: blockService,
	}

	runtime.SetFinalizer(sb, func(o *RustSectorBuilder) {
		o.destroy()
	})

	return sb, nil
}

// AddPiece writes the given piece into an unsealed sector and returns the id
// of that sector.
func (sb *RustSectorBuilder) AddPiece(ctx context.Context, pi *PieceInfo) (sectorID uint64, err error) {
	defer elapsed("AddPiece")()

	pieceKey := pi.Ref.String()
	dagService := dag.NewDAGService(sb.blockService)

	rootIpldNode, err := dagService.Get(ctx, pi.Ref)
	if err != nil {
		return 0, err
	}

	r, err := uio.NewDagReader(ctx, rootIpldNode, dagService)
	if err != nil {
		return 0, err
	}

	pieceBytes := make([]byte, pi.Size)
	_, err = r.Read(pieceBytes)
	if err != nil {
		return 0, errors.Wrapf(err, "error reading piece bytes into buffer")
	}

	cPieceKey := C.CString(pieceKey)
	defer C.free(unsafe.Pointer(cPieceKey))

	cPieceBytes := C.CBytes(pieceBytes)
	defer C.free(cPieceBytes)

	resPtr := (*C.AddPieceResponse)(unsafe.Pointer(C.add_piece(
		(*C.SectorBuilder)(sb.ptr),
		cPieceKey,
		(*C.uint8_t)(cPieceBytes),
		C.size_t(len(pieceBytes)),
	)))
	defer C.destroy_add_piece_response(resPtr)

	if resPtr.status_code != 0 {
		return 0, errors.New(C.GoString(resPtr.error_msg))
	}

	return uint64(resPtr.sector_id), nil
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

// destroy deallocates and destroys a DiskBackedSectorStore.
func (sb *RustSectorBuilder) destroy() {
	C.destroy_sector_builder((*C.SectorBuilder)(sb.ptr))

	sb.ptr = nil
}
