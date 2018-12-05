package sectorbuilder

import (
	"bytes"
	"context"
	"io"
	"runtime"
	"time"
	"unsafe"

	"github.com/filecoin-project/go-filecoin/proofs"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	bserv "gx/ipfs/QmVDTbzzTwnuBwNbJdhW3u7LoBQp46bezm9yp4z1RoEepM/go-blockservice"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	uio "gx/ipfs/QmXAFxWtAB9YAMzMy9op6m95hWYu2CC5rmTsijkYL12Kvu/go-unixfs/io"
	dag "gx/ipfs/QmdURv6Sbob8TVW2tFFve9vcEWrSUgwPqeqnXyvYhLrkyd/go-merkledag"
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

	// sectorSealResults is sent a value whenever seal completes for a sector,
	// either successfully or with a failure.
	sectorSealResults chan SectorSealResult

	// sealStatusPoller polls for sealing status for the sectors whose ids it
	// knows about.
	sealStatusPoller *sealStatusPoller
}

var _ SectorBuilder = &RustSectorBuilder{}

// RustSectorBuilderConfig is a configuration object used when instantiating a
// Rust-backed SectorBuilder through the FFI. All fields are required.
type RustSectorBuilderConfig struct {
	blockService        bserv.BlockService
	lastUsedSectorID    uint64
	metadataDir         string
	proverID            [31]byte
	sealedSectorDir     string
	sectorStoreType     proofs.SectorStoreType
	stagedSectorDir     string
	maxNumStagedSectors int
}

// NewRustSectorBuilder instantiates a SectorBuilder through the FFI.
func NewRustSectorBuilder(cfg RustSectorBuilderConfig) (*RustSectorBuilder, error) {
	defer elapsed("NewRustSectorBuilder")()

	cMetadataDir := C.CString(cfg.metadataDir)
	defer C.free(unsafe.Pointer(cMetadataDir))

	proverIDCBytes := C.CBytes(cfg.proverID[:])
	defer C.free(proverIDCBytes)

	cStagedSectorDir := C.CString(cfg.stagedSectorDir)
	defer C.free(unsafe.Pointer(cStagedSectorDir))

	cSealedSectorDir := C.CString(cfg.sealedSectorDir)
	defer C.free(unsafe.Pointer(cSealedSectorDir))

	scfg, err := proofs.CSectorStoreType(cfg.sectorStoreType)
	if err != nil {
		return nil, errors.Errorf("unknown sector store type: %v", cfg.sectorStoreType)
	}

	resPtr := (*C.InitSectorBuilderResponse)(unsafe.Pointer(C.init_sector_builder(
		(*C.ConfiguredStore)(unsafe.Pointer(scfg)),
		C.uint64_t(cfg.lastUsedSectorID),
		cMetadataDir,
		(*[31]C.uint8_t)(proverIDCBytes),
		cStagedSectorDir,
		cSealedSectorDir,
		C.uint8_t(cfg.maxNumStagedSectors),
	)))
	defer C.destroy_init_sector_builder_response(resPtr)

	if resPtr.status_code != 0 {
		return nil, errors.New(C.GoString(resPtr.error_msg))
	}

	sb := &RustSectorBuilder{
		blockService:      cfg.blockService,
		ptr:               unsafe.Pointer(resPtr.sector_builder),
		sectorSealResults: make(chan SectorSealResult),
	}

	sb.sealStatusPoller = newSealStatusPoller(sb.sectorSealResults, sb.findSealedSectorMetadata)

	runtime.SetFinalizer(sb, func(o *RustSectorBuilder) {
		o.destroy()
	})

	return sb, nil
}

// GetMaxUserBytesPerStagedSector produces the number of user piece-bytes which
// will fit into a newly-provisioned staged sector.
func (sb *RustSectorBuilder) GetMaxUserBytesPerStagedSector() (numBytes uint64, err error) {
	defer elapsed("GetMaxUserBytesPerStagedSector")()

	resPtr := (*C.GetMaxStagedBytesPerSector)(unsafe.Pointer(C.get_max_user_bytes_per_staged_sector((*C.SectorBuilder)(sb.ptr))))
	defer C.destroy_get_max_user_bytes_per_staged_sector_response(resPtr)

	if resPtr.status_code != 0 {
		return 0, errors.New(C.GoString(resPtr.error_msg))
	}

	return uint64(resPtr.max_staged_bytes_per_sector), nil
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

	go sb.sealStatusPoller.addSectorID(uint64(resPtr.sector_id))

	return uint64(resPtr.sector_id), nil
}

func (sb *RustSectorBuilder) findSealedSectorMetadata(sectorID uint64) (*SealedSector, error) {
	resPtr := (*C.GetSealStatusResponse)(unsafe.Pointer(C.get_seal_status((*C.SectorBuilder)(sb.ptr), C.uint64_t(sectorID))))
	defer C.destroy_get_seal_status_response(resPtr)

	if resPtr.status_code != 0 {
		return nil, errors.New(C.GoString(resPtr.error_msg))
	}

	if resPtr.seal_status_code == C.Failed {
		return nil, errors.New(C.GoString(resPtr.seal_error_msg))
	} else if resPtr.seal_status_code == C.Pending {
		return nil, nil
	} else if resPtr.seal_status_code == C.Sealing {
		return nil, nil
	} else if resPtr.seal_status_code == C.Sealed {
		commRSlice := C.GoBytes(unsafe.Pointer(&resPtr.comm_r[0]), 32)
		var commR [32]byte
		copy(commR[:], commRSlice)

		commDSlice := C.GoBytes(unsafe.Pointer(&resPtr.comm_d[0]), 32)
		var commD [32]byte
		copy(commD[:], commDSlice)

		commRStarSlice := C.GoBytes(unsafe.Pointer(&resPtr.comm_r_star[0]), 32)
		var commRStar [32]byte
		copy(commRStar[:], commRStarSlice)

		proofSlice := C.GoBytes(unsafe.Pointer(&resPtr.snark_proof[0]), 384)
		var proof [384]byte
		copy(proof[:], proofSlice)

		ps, err := goPieceInfos((*C.FFIPieceMetadata)(unsafe.Pointer(resPtr.pieces_ptr)), resPtr.pieces_len)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal from string to cid")
		}

		return &SealedSector{
			CommD:              commD,
			CommR:              commR,
			CommRStar:          commRStar,
			pieces:             ps,
			proof:              proof,
			sealedSectorAccess: C.GoString(resPtr.sector_access),
			SectorID:           sectorID,
		}, nil
	} else {
		// unknown
		return nil, errors.New("unexpected seal status")
	}
}

// ReadPieceFromSealedSector produces a Reader used to get original piece-bytes
// from a sealed sector.
func (sb *RustSectorBuilder) ReadPieceFromSealedSector(pieceCid cid.Cid) (io.Reader, error) {
	cPieceKey := C.CString(pieceCid.String())
	defer C.free(unsafe.Pointer(cPieceKey))

	resPtr := (*C.ReadPieceFromSealedSectorResponse)(unsafe.Pointer(C.read_piece_from_sealed_sector((*C.SectorBuilder)(sb.ptr), cPieceKey)))
	defer C.destroy_read_piece_from_sealed_sector_response(resPtr)

	if resPtr.status_code != 0 {
		return nil, errors.New(C.GoString(resPtr.error_msg))
	}

	return bytes.NewReader(C.GoBytes(unsafe.Pointer(resPtr.data_ptr), C.int(resPtr.data_len))), nil
}

// SealAllStagedSectors schedules sealing of all staged sectors.
func (sb *RustSectorBuilder) SealAllStagedSectors(ctx context.Context) error {
	resPtr := (*C.SealAllStagedSectorsResponse)(unsafe.Pointer(C.seal_all_staged_sectors((*C.SectorBuilder)(sb.ptr))))
	defer C.destroy_seal_all_staged_sectors_response(resPtr)

	if resPtr.status_code != 0 {
		return errors.New(C.GoString(resPtr.error_msg))
	}

	return nil
}

// SealedSectors returns a slice of all sealed sector metadata for the sector builder, or an error.
func (sb *RustSectorBuilder) SealedSectors() ([]*SealedSector, error) {
	resPtr := (*C.GetSealedSectorsResponse)(unsafe.Pointer(C.get_sealed_sectors((*C.SectorBuilder)(sb.ptr))))
	defer C.destroy_get_sealed_sectors_response(resPtr)

	if resPtr.status_code != 0 {
		return nil, errors.New(C.GoString(resPtr.error_msg))
	}

	meta, err := goSealedSectorMetadata((*C.FFISealedSectorMetadata)(unsafe.Pointer(resPtr.sectors_ptr)), resPtr.sectors_len)
	if err != nil {
		return nil, err
	}

	return meta, nil
}

// SectorSealResults returns an unbuffered channel that is sent a value whenever
// sealing completes.
func (sb *RustSectorBuilder) SectorSealResults() <-chan SectorSealResult {
	return sb.sectorSealResults
}

// Close shuts down the RustSectorBuilder's poller.
func (sb *RustSectorBuilder) Close() error {
	sb.sealStatusPoller.stop()
	return nil
}

// destroy deallocates and destroys a DiskBackedSectorStore.
func (sb *RustSectorBuilder) destroy() {
	C.destroy_sector_builder((*C.SectorBuilder)(sb.ptr))

	sb.ptr = nil
}

func goSealedSectorMetadata(src *C.FFISealedSectorMetadata, size C.size_t) ([]*SealedSector, error) {
	sectors := make([]*SealedSector, size)
	if src == nil || size == 0 {
		return sectors, nil
	}

	sectorPtrs := (*[1 << 30]C.FFISealedSectorMetadata)(unsafe.Pointer(src))[:size:size]
	for i := 0; i < int(size); i++ {
		secPtr := sectorPtrs[i]

		commRSlice := C.GoBytes(unsafe.Pointer(&secPtr.comm_r[0]), 32)
		var commR [32]byte
		copy(commR[:], commRSlice)

		commDSlice := C.GoBytes(unsafe.Pointer(&secPtr.comm_d[0]), 32)
		var commD [32]byte
		copy(commD[:], commDSlice)

		commRStarSlice := C.GoBytes(unsafe.Pointer(&secPtr.comm_r_star[0]), 32)
		var commRStar [32]byte
		copy(commRStar[:], commRStarSlice)

		proofSlice := C.GoBytes(unsafe.Pointer(&secPtr.snark_proof[0]), 384)
		var proof [384]byte
		copy(proof[:], proofSlice)

		ps, err := goPieceInfos((*C.FFIPieceMetadata)(unsafe.Pointer(secPtr.pieces_ptr)), secPtr.pieces_len)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal from string to cid")
		}

		sectors[i] = &SealedSector{
			CommD:              commD,
			CommR:              commR,
			CommRStar:          commRStar,
			pieces:             ps,
			proof:              proof,
			sealedSectorAccess: C.GoString(secPtr.sector_access),
			SectorID:           uint64(secPtr.sector_id),
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
