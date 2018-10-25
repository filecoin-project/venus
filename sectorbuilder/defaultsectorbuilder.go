package sectorbuilder

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	logging "gx/ipfs/QmRREK2CAZ5Re2Bd9zZFG6FeYDppUWt5cMgsoUEp3ktgSr/go-log"
	bserv "gx/ipfs/QmTfTKeBhTLjSjxXQsjkF2b1DfZmYEMnknGE2y2gX57C6v/go-blockservice"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	ipld "gx/ipfs/QmX5CsuHyVZeTLxgRSYkgLSDQKb9UjE8xnhQzCEJWWWFsC/go-ipld-format"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	uio "gx/ipfs/Qmdg2crJzNUF1mLPnLPSCCaDdLDqE4Qrh9QEiDooSYkvuB/go-unixfs/io"
	dag "gx/ipfs/QmeLG6jF1xvEmHca5Vy4q4EdQWp8Xq9S6EPyZrN9wvSRLC/go-merkledag"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/repo"
)

var log = logging.Logger("sectorbuilder") // nolint: deadcode

// defaultSectorBuilder satisfies the SectorBuilder interface.
type defaultSectorBuilder struct {
	minerAddr address.Address

	curUnsealedSectorLk sync.RWMutex // curUnsealedSectorLk protects curUnsealedSector
	curUnsealedSector   *UnsealedSector
	sealedSectorsLk     sync.RWMutex // sealedSectorsLk protects sealedSectors
	sealedSectors       []*SealedSector

	// SectorSealResults is sent a value whenever sealing completes. The value
	// will be either a *SealedSector or an error.
	sectorSealResults chan interface{}

	// dispenses SectorAccess, used by FPS to determine where to read/write
	// sector and unsealed sector file-bytes
	sectorStore proofs.SectorStore

	// persists and loads metadata
	metadataStore *metadataStore

	// used to stream piece-data to unsealed sector-file
	dserv ipld.DAGService

	// the number of user piece-bytes which can be written to a staged sector
	sectorSize uint64

	sectorIDNonceLk sync.Mutex // sectorIDNonceLk protects sectorIDNonce
	sectorIDNonce   uint64

	// propagated to the sector sealer
	miningCtx context.Context
}

func (sb *defaultSectorBuilder) Close() error {
	close(sb.sectorSealResults)

	return nil
}

func (sb *defaultSectorBuilder) SectorSealResults() <-chan interface{} {
	return sb.sectorSealResults
}

func (sb *defaultSectorBuilder) MaxBytesPerSector() uint64 {
	return sb.sectorSize
}

var _ SectorBuilder = &defaultSectorBuilder{}

// getNextSectorID atomically increments the SectorBuilder's sector ID nonce and returns the incremented value.
func (sb *defaultSectorBuilder) getNextSectorID() uint64 {
	sb.sectorIDNonceLk.Lock()
	defer sb.sectorIDNonceLk.Unlock()

	sb.sectorIDNonce = sb.sectorIDNonce + 1

	return sb.sectorIDNonce
}

// newUnsealedSector allocates and returns a new UnsealedSector with file initialized, along with any error.
func (sb *defaultSectorBuilder) newUnsealedSector() (s *UnsealedSector, err error) {
	res, err := sb.sectorStore.NewStagingSectorAccess()
	if err != nil {
		return nil, errors.Wrap(err, "failed to dispense staging sector unsealedSectorAccess")
	}

	s = &UnsealedSector{
		SectorID:             sb.getNextSectorID(),
		numBytesUsed:         0,
		maxBytes:             sb.sectorSize,
		unsealedSectorAccess: res.SectorAccess,
	}

	s.unsealedSectorAccess = res.SectorAccess

	return s, nil
}

// newSealedSector creates a new SealedSector. The new SealedSector is appended to the slice of sealed sectors managed
// by the SectorBuilder.
func (sb *defaultSectorBuilder) newSealedSector(commR [32]byte, commD [32]byte, proof [384]byte, label, sealedSectorAccess string, s *UnsealedSector) *SealedSector {
	ss := &SealedSector{
		CommD:                commD,
		CommR:                commR,
		numBytes:             s.numBytesUsed,
		pieces:               s.pieces,
		proof:                proof,
		sealedSectorAccess:   sealedSectorAccess,
		SectorID:             s.SectorID,
		unsealedSectorAccess: s.unsealedSectorAccess,
	}

	sb.sealedSectorsLk.Lock()
	sb.sealedSectors = append(sb.sealedSectors, ss)
	sb.sealedSectorsLk.Unlock()

	return ss
}

// SealedSectors returns a list of all currently sealed sectors.
func (sb *defaultSectorBuilder) SealedSectors() []*SealedSector {
	sb.sealedSectorsLk.RLock()
	defer sb.sealedSectorsLk.RUnlock()

	return sb.sealedSectors
}

// Init creates a new sector builder for the given miner. If a SectorBuilder had previously been created
// for the given miner, we reconstruct it using metadata from the datastore so that the miner can resume its work where
// it left off.
func Init(miningCtx context.Context, dataStore repo.Datastore, blockService bserv.BlockService, minerAddr address.Address, sstore proofs.SectorStore, lastUsedSectorID uint64) (SectorBuilder, error) {
	mstore := &metadataStore{
		store: dataStore,
	}

	res, err := sstore.GetMaxUnsealedBytesPerSector()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get number of bytes per sector from store")
	}
	log.Infof("sector size: %d", res.NumBytes)

	sb := &defaultSectorBuilder{
		dserv:             dag.NewDAGService(blockService),
		minerAddr:         minerAddr,
		miningCtx:         miningCtx,
		sectorSize:        res.NumBytes,
		metadataStore:     mstore,
		sectorStore:       sstore,
		sectorIDNonce:     lastUsedSectorID,
		sectorSealResults: make(chan interface{}),
	}

	metadata, err := mstore.getSectorBuilderMetadata(minerAddr)
	if err == nil {
		if err1 := configureSectorBuilderFromMetadata(mstore, sb, metadata); err1 != nil {
			return nil, err1
		}

		sb.curUnsealedSectorLk.RLock()
		unsealedSector := sb.curUnsealedSector
		sb.curUnsealedSectorLk.RUnlock()

		return sb, sb.checkpoint(unsealedSector, nil)
	} else if strings.Contains(err.Error(), "not found") {
		if err1 := configureFreshSectorBuilder(sb); err1 != nil {
			return nil, err1
		}
		sb.curUnsealedSectorLk.RLock()
		unsealedSector := sb.curUnsealedSector
		sb.curUnsealedSectorLk.RUnlock()

		return sb, sb.checkpoint(unsealedSector, nil)
	} else {
		return nil, err
	}
}

func configureSectorBuilderFromMetadata(store *metadataStore, sb *defaultSectorBuilder, metadata *metadata) (finalErr error) {
	sector, err := store.getSector(metadata.CurUnsealedSectorAccess)
	if err != nil {
		return err
	}

	// note: The following guard exists to prevent a situation in which the
	// sector builder was initialized with a sector store configured to use
	// large sectors (FIL_USE_SMALL_SECTORS=false), user data was written to
	// the staging area, and then the node was reconfigured to use small
	// sector sizes (FIL_USE_SMALL_SECTORS=true).
	//
	// Going forward, the SectorBuilder should be able to manage multiple
	// unsealed sectors concurrently, segregating them (and their sealed
	// counterparts) by sector size.
	if sb.MaxBytesPerSector() != sector.maxBytes {
		return errors.Errorf("sector builder has been configured to use %d-byte sectors, but loaded unsealed sector uses %d-byte sectors", sb.MaxBytesPerSector(), sector.maxBytes)
	}

	if err := sb.syncFile(sector); err != nil {
		return errors.Wrapf(err, "failed to sync sector object with unsealed sector %s", sector.unsealedSectorAccess)
	}

	sb.curUnsealedSectorLk.Lock()
	sb.curUnsealedSector = sector
	sb.curUnsealedSectorLk.Unlock()

	for _, commR := range metadata.SealedSectorCommitments {
		sealed, err := store.getSealedSector(commR)
		if err != nil {
			return err
		}

		sb.sealedSectorsLk.Lock()
		sb.sealedSectors = append(sb.sealedSectors, sealed)
		sb.sealedSectorsLk.Unlock()
	}

	return nil
}

func configureFreshSectorBuilder(sb *defaultSectorBuilder) error {
	sb.curUnsealedSectorLk.Lock()
	defer sb.curUnsealedSectorLk.Unlock()

	s, err := sb.newUnsealedSector()
	if err != nil {
		return errors.Wrap(err, "failed to create new unsealed sector")
	}

	sb.curUnsealedSector = s

	return nil
}

// ReadPieceFromSealedSector produces a Reader used to get original piece-bytes from a sealed sector.
func (sb *defaultSectorBuilder) ReadPieceFromSealedSector(pieceCid *cid.Cid) (io.Reader, error) {
	unsealArgs, err := sb.metadataStore.getUnsealArgsForPiece(sb.minerAddr, pieceCid)
	if err != nil {
		return nil, errors.Wrapf(err, "no piece with cid %s has yet been sealed into a sector", pieceCid.String())
	}

	// TODO: Find a way to clean up these temporary files. See message below
	// where we're truncating to 0 bytes. We should probably unseal the full
	// sector (regardless of which piece was requested) to a location which
	// we persist to metadata and then ReadUnsealed from that location.
	res, err := sb.sectorStore.NewStagingSectorAccess()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to dispense new staging sector access")
	}

	res2, err2 := (&proofs.RustProver{}).Unseal(proofs.UnsealRequest{
		NumBytes:    unsealArgs.numBytes,
		OutputPath:  res.SectorAccess,
		ProverID:    addressToProverID(sb.minerAddr),
		SealedPath:  unsealArgs.sealedSectorAccess,
		SectorID:    sectorIDToBytes(unsealArgs.sectorID),
		StartOffset: unsealArgs.startOffset,
		Storage:     sb.sectorStore,
	})
	if err2 != nil {
		return nil, errors.Wrapf(err, "failed to unseal")
	}

	if res2.NumBytesWritten != unsealArgs.numBytes {
		return nil, errors.Errorf("number of bytes written and expected differed - expected: %d, actual: %d", unsealArgs.numBytes, res2.NumBytesWritten)
	}

	res3, err3 := sb.sectorStore.ReadUnsealed(proofs.ReadUnsealedRequest{
		SectorAccess: res.SectorAccess,
		StartOffset:  0,
		NumBytes:     res2.NumBytesWritten,
	})
	if err3 != nil {
		return nil, errors.Wrapf(err, "failed to read unsealed bytes from sector access %s", res.SectorAccess)
	}

	// TODO: This is not an acceptable way to clean up, but rather a stop-
	// gap which meets short-term goals.
	//
	// We should either:
	//
	// 1) delete the unseal target after we're done reading from it or
	// 2) unseal the whole sealed sector to a location which we persist
	//    to metadata (so we can subsequent requests to unseal a sector
	//    do not create new files)
	err4 := sb.sectorStore.TruncateUnsealed(proofs.TruncateUnsealedRequest{
		SectorAccess: res.SectorAccess,
		NumBytes:     0,
	})
	if err4 != nil {
		return nil, errors.Errorf("failed to truncate temporary unseal target to 0 bytes %s", res.SectorAccess)
	}

	return bytes.NewReader(res3.Data), nil
}

// SealAllStagedSectors seals any non-empty staged sectors.
func (sb *defaultSectorBuilder) SealAllStagedSectors(ctx context.Context) error {
	log.Debug("SectorBuilder.SealAllStagedSectors wants sb.curUnsealedSectorLk")
	sb.curUnsealedSectorLk.Lock()

	defer func() {
		log.Debug("SectorBuilder.SealAllStagedSectors relinquishes sb.curUnsealedSectorLk")
		sb.curUnsealedSectorLk.Unlock()
	}()
	log.Debug("SectorBuilder.SealAllStagedSectors got sb.curUnsealedSectorLk")

	if sb.curUnsealedSector.numBytesUsed != 0 {
		if err := sb.sealAsync(sb.curUnsealedSector); err != nil {
			return errors.Wrap(err, "failed to seal sector")
		}

		s, err := sb.newUnsealedSector()
		if err != nil {
			return errors.Wrap(err, "failed to create new unsealed sector during auto-seal")
		}

		sb.curUnsealedSector = s

		if err := sb.checkpoint(sb.curUnsealedSector, nil); err != nil {
			return errors.Wrap(err, "failed to checkpoint during autoseal")
		}
	}

	return nil
}

// AddPiece writes the given piece into an unsealed sector and returns the id of that unsealed sector.
func (sb *defaultSectorBuilder) AddPiece(ctx context.Context, pi *PieceInfo) (sectorID uint64, err error) {
	log.Debugf("SectorBuilder.AddPiece wants sb.curUnsealedSectorLk to add piece %s", pi.Ref.String())
	sb.curUnsealedSectorLk.Lock()
	defer func() {
		log.Debugf("SectorBuilder.AddPiece relinquishes sb.curUnsealedSectorLk for piece %s", pi.Ref.String())
		sb.curUnsealedSectorLk.Unlock()
	}()
	log.Debugf("SectorBuilder.AddPiece got sb.curUnsealedSectorLk to add piece %s", pi.Ref.String())

	ctx = log.Start(ctx, "SectorBuilder.AddPiece")
	log.SetTag(ctx, "piece", pi)
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	if pi.Size > sb.sectorSize {
		return 0, ErrPieceTooLarge
	}

	// if the piece can't fit into the current unsealed sector, seal the
	// current unsealed sector
	if pi.Size > sb.curUnsealedSector.maxBytes-sb.curUnsealedSector.numBytesUsed {
		if err := sb.sealAsync(sb.curUnsealedSector); err != nil {
			return 0, errors.Wrap(err, "failed to seal sector")
		}

		unsealedSector, err := sb.newUnsealedSector()
		if err != nil {
			return 0, errors.Wrap(err, "failed to create new unsealed sector")
		}
		sb.curUnsealedSector = unsealedSector
	}

	// write piece-bytes to the current unsealed sector
	if err := sb.writePiece(ctx, sb.curUnsealedSector, pi); err != nil {
		// If, during piece-writing, a greater-than-zero-amount of piece-bytes were
		// written to the unsealed sector file and we were unable to revert to the
		// pre-write state, ErrCouldNotRevertUnsealedSector will be returned. If we
		// were unable to revert, it is likely that the sector object and backing
		// unsealed sector-file are now in different states.
		if _, ok := err.(*ErrCouldNotRevertUnsealedSector); ok {
			panic(err)
		}

		return 0, errors.Wrap(err, "failed to write piece")
	}

	// capture the id of the sector to which we wrote piece bytes
	pieceAddedToSectorID := sb.curUnsealedSector.SectorID

	// if we just reduced the free space in the unsealed sector to 0,
	// go ahead and seal it
	if sb.curUnsealedSector.numBytesUsed == sb.curUnsealedSector.maxBytes {
		if err := sb.sealAsync(sb.curUnsealedSector); err != nil {
			return 0, errors.Wrap(err, "failed to seal sector")
		}

		unsealedSector, err := sb.newUnsealedSector()
		if err != nil {
			return 0, errors.Wrap(err, "failed to create new unsealed sector")
		}
		sb.curUnsealedSector = unsealedSector
	}

	// checkpoint after we've added the piece and updated the sector builder's
	// "current sector"
	if err := sb.checkpoint(sb.curUnsealedSector, nil); err != nil {
		return 0, err
	}

	return pieceAddedToSectorID, nil
}

// syncFile synchronizes the sector object and backing unsealed sector-file. syncFile may mutate both the file and the
// sector object in order to achieve a consistent view of the sector.
// TODO: We should probably move most of this logic into Rust.
func (sb *defaultSectorBuilder) syncFile(s *UnsealedSector) error {
	res, err := sb.sectorStore.GetNumBytesUnsealed(proofs.GetNumBytesUnsealedRequest{
		SectorAccess: s.unsealedSectorAccess,
	})
	if err != nil {
		return errors.Wrapf(err, "failed to get number of bytes for unsealed sector %s", s.unsealedSectorAccess)
	}

	// | file size | metadata pieces | action                                        |
	// |-----------|-----------------|-----------------------------------------------|
	// | 500 bytes | 100|100|100     | truncate file to 300 bytes                    |
	// | 500 bytes | 100|100|400     | truncate file to 200 bytes, pieces to 100|100 |
	// | 500 bytes | 100|400         | noop                                          |

	cmpSize := func(s *UnsealedSector, fileSize uint64) int {
		if s.numBytesUsed == fileSize {
			return 0
		}
		if s.numBytesUsed < fileSize {
			return -1
		}
		return +1
	}

	// remove pieces from the sector until (s.numBytesUsed-s.numBytesFree) <= fi.size()
	for i := len(s.pieces) - 1; i >= 0; i-- {
		if cmpSize(s, res.NumBytes) == 1 {
			s.numBytesUsed -= s.pieces[i].Size
			s.pieces = s.pieces[:len(s.pieces)-1]
		} else {
			break
		}
	}

	if cmpSize(s, res.NumBytes) == -1 {
		return sb.sectorStore.TruncateUnsealed(proofs.TruncateUnsealedRequest{
			SectorAccess: s.unsealedSectorAccess,
			NumBytes:     s.numBytesUsed,
		})
	}

	return nil
}

// writePiece writes piece bytes to an unsealed sector.
func (sb *defaultSectorBuilder) writePiece(ctx context.Context, dest *UnsealedSector, pi *PieceInfo) (finalErr error) {
	ctx = log.Start(ctx, "UnsealedSector.writePiece")
	defer func() {
		log.FinishWithErr(ctx, finalErr)
	}()

	root, err := sb.dserv.Get(ctx, pi.Ref)
	if err != nil {
		return err
	}

	r, err := uio.NewDagReader(ctx, root, sb.dserv)
	if err != nil {
		return err
	}

	// TODO: This is a temporary workaround. Once we complete rust-proofs issue
	// 140, we can replace this buffer with a streaming implementation.
	b := make([]byte, pi.Size)
	n, err := r.Read(b)
	if err != nil {
		return errors.Wrapf(err, "error reading piece bytes into buffer")
	}

	if uint64(n) != pi.Size {
		return fmt.Errorf("could not read all piece-bytes to buffer (pi.size=%d, read=%d)", pi.Size, n)
	}

	res, err := sb.sectorStore.WriteUnsealed(proofs.WriteUnsealedRequest{
		SectorAccess: dest.unsealedSectorAccess,
		Data:         b,
	})
	if err != nil {
		return errors.Wrapf(err, "failed to write bytes to unsealed sector %s", dest.unsealedSectorAccess)
	}

	dest.numBytesUsed += pi.Size
	dest.pieces = append(dest.pieces, pi)

	// NumBytesWritten can be larger, due to padding
	if res.NumBytesWritten < pi.Size {
		err := fmt.Errorf("did not write all piece-bytes to file (pi.size=%d, wrote=%d)", pi.Size, res.NumBytesWritten)

		if err1 := sb.syncFile(dest); err1 != nil {
			return NewErrCouldNotRevertUnsealedSector(err1, err)
		}

		return err
	}

	return nil
}

// sealAsync kicks off the generation of a proof of replication. This method
// updates related metadata both before and after the FPS seal operation
// completes.
func (sb *defaultSectorBuilder) sealAsync(s *UnsealedSector) (finalErr error) {
	if err := sb.checkpoint(s, nil); err != nil {
		return errors.Wrap(err, "failed to create checkpoint")
	}

	res1, err := sb.sectorStore.NewSealedSectorAccess()
	if err != nil {
		return errors.Wrap(err, "failed to dispense sealed sector unsealedSectorAccess")
	}

	req := proofs.SealRequest{
		ProverID:     addressToProverID(sb.minerAddr),
		SealedPath:   res1.SectorAccess,
		SectorID:     sectorIDToBytes(s.SectorID),
		Storage:      sb.sectorStore,
		UnsealedPath: s.unsealedSectorAccess,
	}

	go func() {
		res2, err := (&proofs.RustProver{}).Seal(req)
		if err != nil {
			sb.sectorSealResults <- err
			return
		}

		ss := sb.newSealedSector(res2.CommR, res2.CommD, res2.Proof, res1.SectorAccess, res1.SectorAccess, s)
		if err := sb.checkpoint(s, ss); err != nil {
			log.Errorf("failed to checkpoint the recently-sealed, staged sector with id %d: %s", s.SectorID, err.Error())
		}

		sb.sectorSealResults <- ss
	}()

	return nil
}

// TODO: Sealed sector metadata and sector metadata shouldn't exist in the
// datastore at the same time, and sector builder metadata needs to be kept
// in sync with sealed sector metadata (e.g. which sectors are sealed).
// This is the method to enforce these rules. Unfortunately this means that
// we're making more writes to the datastore than we really need to be
// doing. As the SectorBuilder evolves, we will introduce some checks which
// will optimize away redundant writes to the datastore.
func (sb *defaultSectorBuilder) checkpoint(s *UnsealedSector, ss *SealedSector) error {
	if err := sb.metadataStore.setSectorBuilderMetadata(sb.minerAddr, sb.dumpCurrentState()); err != nil {
		return errors.Wrap(err, "failed to save builder metadata")
	}

	if ss == nil {
		if err := sb.metadataStore.setSectorMetadata(s.unsealedSectorAccess, s.dumpCurrentState()); err != nil {
			return errors.Wrap(err, "failed to save sector metadata")
		}
	} else {
		if err := sb.metadataStore.setSealedSectorMetadata(ss.CommR, ss.dumpCurrentState()); err != nil {
			return errors.Wrap(err, "failed to save sealed sector metadata")
		}

		if err := sb.metadataStore.deleteSectorMetadata(s.unsealedSectorAccess); err != nil {
			return errors.Wrap(err, "failed to remove sector metadata")
		}
	}

	return nil
}
