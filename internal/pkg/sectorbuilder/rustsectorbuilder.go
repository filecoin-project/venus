package sectorbuilder

import (
	"bytes"
	"context"
	"io"
	"unsafe"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/sectorbuilder/bytesink"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-sectorbuilder"
)

var log = logging.Logger("rustsectorbuilder") // nolint: deadcode

// MaxNumStagedSectors configures the maximum number of staged sectors which can
// be open and accepting data at any time.
const MaxNumStagedSectors = 1

// RustSectorBuilder is a struct which serves as a proxy for a SectorBuilder in Rust.
type RustSectorBuilder struct {
	ptr unsafe.Pointer

	// sectorSealResults is sent a value whenever seal completes for a sector,
	// either successfully or with a failure.
	sectorSealResults chan SectorSealResult

	// sealStatusPoller polls for sealing status for the sectors whose ids it
	// knows about.
	sealStatusPoller *sealStatusPoller

	// SectorClass configures behavior of sector_builder_ffi, including sector
	// packing, sector sizes, sealing and PoSt generation performance.
	SectorClass types.SectorClass
}

var _ SectorBuilder = &RustSectorBuilder{}

// RustSectorBuilderConfig is a configuration object used when instantiating a
// Rust-backed SectorBuilder through the FFI. All fields are required.
type RustSectorBuilderConfig struct {
	LastUsedSectorID uint64
	MetadataDir      string
	MinerAddr        address.Address
	SealedSectorDir  string
	StagedSectorDir  string
	SectorClass      types.SectorClass
}

// NewRustSectorBuilder instantiates a SectorBuilder through the FFI.
func NewRustSectorBuilder(cfg RustSectorBuilderConfig) (*RustSectorBuilder, error) {
	ptr, err := go_sectorbuilder.InitSectorBuilder(cfg.SectorClass.SectorSize().Uint64(), uint8(cfg.SectorClass.PoRepProofPartitions().Int()), uint8(cfg.SectorClass.PoStProofPartitions().Int()), cfg.LastUsedSectorID, cfg.MetadataDir, AddressToProverID(cfg.MinerAddr), cfg.SealedSectorDir, cfg.StagedSectorDir, MaxNumStagedSectors)
	if err != nil {
		return nil, err
	}

	sb := &RustSectorBuilder{
		ptr:               ptr,
		sectorSealResults: make(chan SectorSealResult),
		SectorClass:       cfg.SectorClass,
	}

	// load staged sector metadata and use it to initialize the poller
	metadata, err := sb.GetAllStagedSectors()
	if err != nil {
		return nil, errors.Wrap(err, "failed to load staged sectors")
	}

	stagedSectorIDs := make([]uint64, len(metadata))
	for idx, m := range metadata {
		stagedSectorIDs[idx] = m.SectorID
	}

	sb.sealStatusPoller = newSealStatusPoller(stagedSectorIDs, sb.sectorSealResults, sb.findSealedSectorMetadata)

	return sb, nil
}

// AddPiece writes the given piece into an unsealed sector and returns the id
// of that sector.
func (sb *RustSectorBuilder) AddPiece(ctx context.Context, pieceRef cid.Cid, pieceSize uint64, pieceReader io.Reader) (sectorID uint64, retErr error) {
	fifoFile, err := bytesink.NewFifo()
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

	// goroutine attempts to copy bytes from piece's reader to the fifoFile
	go func() {
		// opening the fifoFile blocks the goroutine until a reader is opened on the
		// other end of the FIFO pipe
		err := fifoFile.Open()
		if err != nil {
			errCh <- errors.Wrap(err, "failed to open fifoFile")
			return
		}

		// closing the fifoFile signals to the reader that we're done writing, which
		// unblocks the reader
		defer func() {
			err := fifoFile.Close()
			if err != nil {
				log.Warnf("failed to close fifoFile: %s", err)
			}
		}()

		n, err := io.Copy(fifoFile, pieceReader)
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
		id, err := go_sectorbuilder.AddPiece(sb.ptr, pieceRef.String(), pieceSize, fifoFile.ID())
		if err != nil {
			msg := "CGO add_piece returned an error (err=%s, fifo path=%s)"
			log.Errorf(msg, err, fifoFile.ID())
			errCh <- err
			return
		}

		sectorIDCh <- id
	}()

	select {
	case <-ctx.Done():
		errStr := "context completed before CGO call could return"
		strFmt := "%s (sinkPath=%s)"
		log.Errorf(strFmt, errStr, fifoFile.ID())

		return 0, errors.New(errStr)
	case err := <-errCh:
		errStr := "error streaming piece-bytes"
		strFmt := "%s (sinkPath=%s)"
		log.Errorf(strFmt, errStr, fifoFile.ID())

		return 0, errors.Wrap(err, errStr)
	case sectorID := <-sectorIDCh:
		go sb.sealStatusPoller.addSectorID(sectorID)
		log.Infof("add piece complete (pieceRef=%s, sectorID=%d, sinkPath=%s)", pieceRef.String(), sectorID, fifoFile.ID())

		return sectorID, nil
	}
}

func (sb *RustSectorBuilder) findSealedSectorMetadata(sectorID uint64) (*SealedSectorMetadata, error) {
	status, err := go_sectorbuilder.GetSectorSealingStatusByID(sb.ptr, sectorID)
	if err != nil {
		return nil, err
	}

	if status.SealStatusCode == 0 {
		info := make([]*PieceInfo, len(status.Pieces))
		for idx, pieceMetadata := range status.Pieces {
			p := &PieceInfo{
				Size:           pieceMetadata.Size,
				InclusionProof: pieceMetadata.InclusionProof,
				CommP:          pieceMetadata.CommP,
			}

			// decode piece key-string to CID
			ref, err := cid.Decode(pieceMetadata.Key)
			if err != nil {
				return nil, err
			}
			p.Ref = ref

			info[idx] = p
		}

		// complete
		return &SealedSectorMetadata{
			CommD:     status.CommD,
			CommR:     status.CommR,
			CommRStar: status.CommRStar,
			Pieces:    info,
			Proof:     status.Proof,
			SectorID:  status.SectorID,
		}, nil
	} else if status.SealStatusCode == 1 || status.SealStatusCode == 3 {
		// staged or currently being sealed
		return nil, nil
	} else if status.SealStatusCode == 2 {
		// failed
		return nil, errors.New(status.SealErrorMsg)
	} else {
		// unknown
		return nil, errors.New("unexpected seal status")
	}
}

// ReadPieceFromSealedSector produces a Reader used to get original piece-bytes
// from a sealed sector.
func (sb *RustSectorBuilder) ReadPieceFromSealedSector(pieceCid cid.Cid) (io.Reader, error) {
	buffer, err := go_sectorbuilder.ReadPieceFromSealedSector(sb.ptr, pieceCid.String())
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(buffer), err
}

// SealAllStagedSectors schedules sealing of all staged sectors.
func (sb *RustSectorBuilder) SealAllStagedSectors(ctx context.Context) error {
	return go_sectorbuilder.SealAllStagedSectors(sb.ptr)
}

// GetAllStagedSectors returns a slice of all staged sector metadata for the sector builder, or an error.
func (sb *RustSectorBuilder) GetAllStagedSectors() ([]go_sectorbuilder.StagedSectorMetadata, error) {
	original, err := go_sectorbuilder.GetAllStagedSectors(sb.ptr)
	if err != nil {
		return nil, err
	}

	// NOTE: Omitting staged sector metadata from the output slice is a hacky
	// workaround for the rust-fil-sector-builder/75 bug. This bug is now
	// patched in rust-fil-sector-builder, but go-filecoin is using a very old
	// version of go-sectorbuilder (and thus rust-fil-sector-builder).
	//
	// For more details, see:
	// * https://github.com/filecoin-project/rust-fil-sector-builder/issues/75
	// * https://github.com/filecoin-project/go-filecoin/issues/3479
	//
	var scrubbed []go_sectorbuilder.StagedSectorMetadata
	for _, meta := range original {
		status, err := go_sectorbuilder.GetSectorSealingStatusByID(sb.ptr, meta.SectorID)
		if err != nil {
			return nil, err
		}

		// if the sector has been sealed, don't add it to the staged sectors
		// output-slice
		if status.SealStatusCode != 0 {
			scrubbed = append(scrubbed, meta)
		}
	}

	return scrubbed, nil
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
	go_sectorbuilder.DestroySectorBuilder(sb.ptr)
	sb.ptr = nil

	return nil
}

// GeneratePoSt produces a proof-of-spacetime for the provided replica commitments.
func (sb *RustSectorBuilder) GeneratePoSt(req GeneratePoStRequest) (GeneratePoStResponse, error) {
	proof, err := go_sectorbuilder.GeneratePoSt(sb.ptr, req.SortedSectorInfo, req.ChallengeSeed, []uint64{})
	if err != nil {
		return GeneratePoStResponse{}, err
	}

	return GeneratePoStResponse{
		Proof: proof,
	}, nil
}
