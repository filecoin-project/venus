// +build !windows

package sectorbuilder

import (
	"bytes"
	"context"
	"io"
	"time"
	"unsafe"

	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/proofs/sectorbuilder/bytesink"
	"github.com/filecoin-project/go-filecoin/types"
)

// #cgo LDFLAGS: -L${SRCDIR}/../lib -lfilecoin_proofs
// #cgo pkg-config: ${SRCDIR}/../lib/pkgconfig/libfilecoin_proofs.pc
// #include "../include/libfilecoin_proofs.h"
import "C"

var log = logging.Logger("sectorbuilder") // nolint: deadcode

// MaxNumStagedSectors configures the maximum number of staged sectors which can
// be open and accepting data at any time.
const MaxNumStagedSectors = 1

// stagedSectorMetadata is a sector into which we write user piece-data before
// sealing. Note: sectorID is unique across all staged and sealed sectors for a
// miner.
type stagedSectorMetadata struct {
	sectorID uint64
}

func elapsed(what string) func() {
	start := time.Now()
	return func() {
		log.Debugf("%s took %v\n", what, time.Since(start))
	}
}

// RustSectorBuilder is a struct which serves as a proxy for a SectorBuilder in Rust.
type RustSectorBuilder struct {
	blockService bserv.BlockService
	ptr          unsafe.Pointer

	// sectorSealResults is sent a value whenever seal completes for a sector,
	// either successfully or with a failure.
	sectorSealResults chan SectorSealResult

	// sealStatusPoller polls for sealing status for the sectors whose ids it
	// knows about.
	sealStatusPoller *sealStatusPoller

	// SectorClass configures behavior of libfilecoin_proofs, including sector
	// packing, sector sizes, sealing and PoSt generation performance.
	SectorClass types.SectorClass
}

var _ SectorBuilder = &RustSectorBuilder{}

// RustSectorBuilderConfig is a configuration object used when instantiating a
// Rust-backed SectorBuilder through the FFI. All fields are required.
type RustSectorBuilderConfig struct {
	BlockService     bserv.BlockService
	LastUsedSectorID uint64
	MetadataDir      string
	MinerAddr        address.Address
	SealedSectorDir  string
	StagedSectorDir  string
	SectorClass      types.SectorClass
}

// NewRustSectorBuilder instantiates a SectorBuilder through the FFI.
func NewRustSectorBuilder(cfg RustSectorBuilderConfig) (*RustSectorBuilder, error) {
	defer elapsed("NewRustSectorBuilder")()

	cMetadataDir := C.CString(cfg.MetadataDir)
	defer C.free(unsafe.Pointer(cMetadataDir))

	proverID := AddressToProverID(cfg.MinerAddr)

	proverIDCBytes := C.CBytes(proverID[:])
	defer C.free(proverIDCBytes)

	cStagedSectorDir := C.CString(cfg.StagedSectorDir)
	defer C.free(unsafe.Pointer(cStagedSectorDir))

	cSealedSectorDir := C.CString(cfg.SealedSectorDir)
	defer C.free(unsafe.Pointer(cSealedSectorDir))

	class, err := cSectorClass(cfg.SectorClass)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get sector class")
	}

	resPtr := (*C.InitSectorBuilderResponse)(unsafe.Pointer(C.init_sector_builder(
		class,
		C.uint64_t(cfg.LastUsedSectorID),
		cMetadataDir,
		(*[31]C.uint8_t)(proverIDCBytes),
		cSealedSectorDir,
		cStagedSectorDir,
		C.uint8_t(MaxNumStagedSectors),
	)))
	defer C.destroy_init_sector_builder_response(resPtr)

	if resPtr.status_code != 0 {
		return nil, errors.New(C.GoString(resPtr.error_msg))
	}

	sb := &RustSectorBuilder{
		blockService:      cfg.BlockService,
		ptr:               unsafe.Pointer(resPtr.sector_builder),
		sectorSealResults: make(chan SectorSealResult),
		SectorClass:       cfg.SectorClass,
	}

	// load staged sector metadata and use it to initialize the poller
	metadata, err := sb.stagedSectors()
	if err != nil {
		return nil, errors.Wrap(err, "failed to load staged sectors")
	}

	stagedSectorIDs := make([]uint64, len(metadata))
	for idx, m := range metadata {
		stagedSectorIDs[idx] = m.sectorID
	}

	sb.sealStatusPoller = newSealStatusPoller(stagedSectorIDs, sb.sectorSealResults, sb.findSealedSectorMetadata)

	return sb, nil
}

// AddPiece writes the given piece into an unsealed sector and returns the id
// of that sector.
func (sb *RustSectorBuilder) AddPiece(ctx context.Context, pieceRef cid.Cid, pieceSize uint64, pieceReader io.Reader) (sectorID uint64, retErr error) {
	defer elapsed("AddPiece")()

	sink, err := bytesink.NewFifo()
	if err != nil {
		return 0, err
	}

	// errCh holds any error encountered when streaming bytes or making the CGO
	// call. The channel is buffered so that the goroutines can exit, which will
	// close the pipe, which unblocks the CGO call.
	errCh := make(chan error, 2)

	// sectorIDCh receives a value if the CGO call indicates that the client
	// piece has successfully been added to a sector. The channel is buffered
	// so that the goroutine can exit if a value is sent to errCh before the
	// CGO call completes.
	sectorIDCh := make(chan uint64, 1)

	// goroutine attempts to copy bytes from piece's reader to the sink
	go func() {
		// opening the sink blocks the goroutine until a reader is opened on the
		// other end of the FIFO pipe
		err := sink.Open()
		if err != nil {
			errCh <- errors.Wrap(err, "failed to open sink")
			return
		}

		// closing the sink signals to the reader that we're done writing, which
		// unblocks the reader
		defer func() {
			err := sink.Close()
			if err != nil {
				log.Warningf("failed to close sink: %s", err)
			}
		}()

		n, err := io.Copy(sink, pieceReader)
		if err != nil {
			errCh <- errors.Wrap(err, "failed to copy to pipe")
			return
		}

		if uint64(n) != pieceSize {
			errCh <- errors.Errorf("expected to write %d bytes but wrote %d", pieceSize, n)
			return
		}
	}()

	// goroutine makes CGO call, which blocks until FIFO pipe opened for writing
	// from within other goroutine
	go func() {
		cPieceKey := C.CString(pieceRef.String())
		defer C.free(unsafe.Pointer(cPieceKey))

		cSinkPath := C.CString(sink.ID())
		defer C.free(unsafe.Pointer(cSinkPath))

		resPtr := (*C.AddPieceResponse)(unsafe.Pointer(C.add_piece(
			(*C.SectorBuilder)(sb.ptr),
			cPieceKey,
			C.uint64_t(pieceSize),
			cSinkPath,
		)))
		defer C.destroy_add_piece_response(resPtr)

		if resPtr.status_code != 0 {
			msg := "CGO add_piece returned an error (error_msg=%s, sinkPath=%s)"
			log.Errorf(msg, C.GoString(resPtr.error_msg), sink.ID())
			errCh <- errors.New(C.GoString(resPtr.error_msg))
			return
		}

		sectorIDCh <- uint64(resPtr.sector_id)
	}()

	select {
	case <-ctx.Done():
		errStr := "context completed before CGO call could return"
		strFmt := "%s (sinkPath=%s)"
		log.Errorf(strFmt, errStr, sink.ID())

		return 0, errors.New(errStr)
	case err := <-errCh:
		errStr := "error streaming piece-bytes"
		strFmt := "%s (sinkPath=%s)"
		log.Errorf(strFmt, errStr, sink.ID())

		return 0, errors.Wrap(err, errStr)
	case sectorID := <-sectorIDCh:
		go sb.sealStatusPoller.addSectorID(sectorID)
		log.Infof("add piece complete (pieceRef=%s, sectorID=%d, sinkPath=%s)", pieceRef.String(), sectorID, sink.ID())

		return sectorID, nil
	}
}

func (sb *RustSectorBuilder) findSealedSectorMetadata(sectorID uint64) (*SealedSectorMetadata, error) {
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
		commRSlice := goBytes(&resPtr.comm_r[0], 32)
		var commR types.CommR
		copy(commR[:], commRSlice)

		commDSlice := goBytes(&resPtr.comm_d[0], 32)
		var commD types.CommD
		copy(commD[:], commDSlice)

		commRStarSlice := goBytes(&resPtr.comm_r_star[0], 32)
		var commRStar types.CommRStar
		copy(commRStar[:], commRStarSlice)

		proof := goBytes(resPtr.proof_ptr, resPtr.proof_len)

		ps, err := goPieceInfos(resPtr.pieces_ptr, resPtr.pieces_len)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal from string to cid")
		}

		// TODO: These piece inclusion proofs are fake, remove this when proofs are available
		// The fake proof uses the piece cid as a fake CommP and concatenates CommP with CommD
		// see https://github.com/filecoin-project/go-filecoin/issues/2629
		for _, pieceInfo := range ps {
			var commP types.CommP
			copy(commP[:], pieceInfo.Ref.Bytes())
			pieceInfo.InclusionProof = []byte{}
			pieceInfo.InclusionProof = append(pieceInfo.InclusionProof, commP[:]...)
			pieceInfo.InclusionProof = append(pieceInfo.InclusionProof, commD[:]...)
		}

		return &SealedSectorMetadata{
			CommD:     commD,
			CommR:     commR,
			CommRStar: commRStar,
			Pieces:    ps,
			Proof:     proof,
			SectorID:  sectorID,
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

	return bytes.NewReader(goBytes(resPtr.data_ptr, resPtr.data_len)), nil
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

// stagedSectors returns a slice of all staged sector metadata for the sector builder, or an error.
func (sb *RustSectorBuilder) stagedSectors() ([]*stagedSectorMetadata, error) {
	resPtr := (*C.GetStagedSectorsResponse)(unsafe.Pointer(C.get_staged_sectors((*C.SectorBuilder)(sb.ptr))))
	defer C.destroy_get_staged_sectors_response(resPtr)

	if resPtr.status_code != 0 {
		return nil, errors.New(C.GoString(resPtr.error_msg))
	}

	meta, err := goStagedSectorMetadata((*C.FFIStagedSectorMetadata)(unsafe.Pointer(resPtr.sectors_ptr)), resPtr.sectors_len)
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

// Close closes the sector builder and deallocates its (Rust) memory, rendering
// it unusable for I/O.
func (sb *RustSectorBuilder) Close() error {
	sb.sealStatusPoller.stop()
	C.destroy_sector_builder((*C.SectorBuilder)(sb.ptr))
	sb.ptr = nil

	return nil
}

// GeneratePoSt produces a proof-of-spacetime for the provided commitment replicas.
func (sb *RustSectorBuilder) GeneratePoSt(req GeneratePoStRequest) (GeneratePoStResponse, error) {
	defer elapsed("GeneratePoSt")()

	// flattening the byte slice makes it easier to copy into the C heap
	commRs := req.SortedCommRs.Values()
	flattened := make([]byte, 32*len(commRs))
	for idx, commR := range commRs {
		copy(flattened[(32*idx):(32*(1+idx))], commR[:])
	}

	// copy the Go byte slice into C memory
	cflattened := C.CBytes(flattened)
	defer C.free(cflattened)

	challengeSeedPtr := unsafe.Pointer(&(req.ChallengeSeed)[0])

	// a mutable pointer to a GeneratePoStResponse C-struct
	resPtr := (*C.GeneratePoStResponse)(unsafe.Pointer(C.generate_post((*C.SectorBuilder)(sb.ptr), (*C.uint8_t)(cflattened), C.size_t(len(flattened)), (*[32]C.uint8_t)(challengeSeedPtr))))
	defer C.destroy_generate_post_response(resPtr)

	if resPtr.status_code != 0 {
		return GeneratePoStResponse{}, errors.New(C.GoString(resPtr.error_msg))
	}

	proofs, err := goPoStProofs(resPtr.proof_partitions, resPtr.flattened_proofs_ptr, resPtr.flattened_proofs_len)
	if err != nil {
		return GeneratePoStResponse{}, errors.Wrap(err, "failed to convert to []PoStProof")
	}

	return GeneratePoStResponse{
		Proofs: proofs,
		Faults: goUint64s(resPtr.faults_ptr, resPtr.faults_len),
	}, nil
}
