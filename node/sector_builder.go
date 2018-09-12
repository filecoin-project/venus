package node

import (
	"bytes"
	"context"
	"encoding/base32"
	"fmt"
	"io"
	"math/big"
	"strings"
	"sync"

	cbor "gx/ipfs/QmV6BQ6fFCf9eFHDuRxvguvqfKLZtZrxthgZvDfRCs4tMN/go-ipld-cbor"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	ipld "gx/ipfs/QmX5CsuHyVZeTLxgRSYkgLSDQKb9UjE8xnhQzCEJWWWFsC/go-ipld-format"
	cid "gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	uio "gx/ipfs/Qmdg2crJzNUF1mLPnLPSCCaDdLDqE4Qrh9QEiDooSYkvuB/go-unixfs/io"
	dag "gx/ipfs/QmeLG6jF1xvEmHca5Vy4q4EdQWp8Xq9S6EPyZrN9wvSRLC/go-merkledag"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/util/binpack"
)

func init() {
	cbor.RegisterCborType(PieceInfo{})
}

// ErrPieceTooLarge is an error indicating that a piece cannot be larger than the sector into which it is written.
var ErrPieceTooLarge = errors.New("piece too large for sector")

// ErrCouldNotRevertUnsealedSector is an error indicating that a revert of an unsealed sector failed due to
// rollbackErr. This revert was originally triggered by the rollbackCause error
type ErrCouldNotRevertUnsealedSector struct {
	rollbackErr   error
	rollbackCause error
}

// NewErrCouldNotRevertUnsealedSector produces an ErrCouldNotRevertUnsealedSector.
func NewErrCouldNotRevertUnsealedSector(rollbackErr error, rollbackCause error) error {
	return &ErrCouldNotRevertUnsealedSector{
		rollbackErr:   rollbackErr,
		rollbackCause: rollbackCause,
	}
}

func (e *ErrCouldNotRevertUnsealedSector) Error() string {
	return fmt.Sprintf("rollback error: %s, rollback cause: %s", e.rollbackErr.Error(), e.rollbackCause.Error())
}

const sectorSize = 1024

var noSectorID = big.NewInt(-1)

// SectorDirs describes the methods required to supply sector directories to a SectorBuilder.
type SectorDirs interface {
	StagingDir() string
	SealedDir() string
}

// PieceInfo is information about a filecoin piece
type PieceInfo struct {
	Ref    *cid.Cid `json:"ref"`
	Size   uint64   `json:"size"`
	DealID uint64   `json:"deal_id"`
}

// SectorBuilder manages packing deals into sectors
// maybe this belongs somewhere else as part of a different thing?
type SectorBuilder struct {
	MinerAddr address.Address

	curUnsealedSectorLk sync.RWMutex // curUnsealedSectorLk protects curUnsealedSector
	curUnsealedSector   *UnsealedSector
	sealedSectorsLk     sync.RWMutex // sealedSectorsLk protects sealedSectors
	sealedSectors       []*SealedSector

	// coordinates opening (creating), packing (writing to), and closing
	// (sealing) sectors
	Packer binpack.Packer

	// OnCommitmentAddedToMempool is called when a sector has been sealed
	// and its commitment added to the message pool.
	OnCommitmentAddedToMempool func(*SealedSector, *cid.Cid, error)

	// yada yada don't hold a reference to this here, just take what you need
	nd *Node

	// dispenses SectorAccess, used by FPS to determine where to read/write
	// sector and unsealed sector file-bytes
	sectorStore proofs.SectorStore

	// persists and loads metadata
	metadataStore *sectorMetadataStore

	// used to stream piece-data to unsealed sector-file
	dserv ipld.DAGService

	sectorSize uint64
}

var _ binpack.Binner = &SectorBuilder{}

// UnsealedSector is a filecoin storage sector. A miner fills this up with data, and
// then seals it. The first part I can do, the second part needs to be figured out more.
// Somehow, I turn this piecemap and backing data buffer into something that the chain can verify.
// So something X has to be computed from this that convinces the chain that
// this miner is storing all the deals referenced in the piece map...
type UnsealedSector struct {
	numBytesFree         uint64 // equals sector bytes - bytes used
	numBytesUsed         uint64
	pieces               []*PieceInfo
	sealed               *SealedSector
	unsealedSectorAccess string
}

var _ binpack.Bin = &UnsealedSector{}

// SealedSector is a sector that has been sealed by the PoRep setup process
type SealedSector struct {
	commD                [32]byte
	commR                [32]byte
	numBytes             uint64
	pieces               []*PieceInfo
	proof                [192]byte
	sealedSectorAccess   string
	unsealedSectorAccess string
}

// GetCurrentBin implements Binner.
func (sb *SectorBuilder) GetCurrentBin() binpack.Bin {
	sb.curUnsealedSectorLk.RLock()
	defer sb.curUnsealedSectorLk.RUnlock()

	return sb.curUnsealedSector
}

// AddItem implements binpack.Binner.
func (sb *SectorBuilder) AddItem(ctx context.Context, item binpack.Item, bin binpack.Bin) (err error) {
	ctx = log.Start(ctx, "SectorBuilder.AddItem")
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	pi := item.(*PieceInfo)
	s := bin.(*UnsealedSector)
	root, err := sb.dserv.Get(ctx, pi.Ref)
	if err != nil {
		return err
	}

	r, err := uio.NewDagReader(ctx, root, sb.dserv)
	if err != nil {
		return err
	}

	if err := sb.WritePiece(ctx, s, pi, r); err != nil {
		return err
	}

	return nil
}

// CloseBin implements binpack.Binner.
func (sb *SectorBuilder) CloseBin(bin binpack.Bin) {
	// TODO: This should be rewritten to allow the caller to control
	// concurrency (either by creating the goroutine or providing a callback
	// function).
	go func() {
		sector := bin.(*UnsealedSector)
		msgCid, err := sb.SealAndAddCommitmentToMempool(context.Background(), sector)
		if err != nil {
			sb.OnCommitmentAddedToMempool(nil, nil, errors.Wrap(err, "failed to seal and commit sector"))
			return
		}

		// TODO: maybe send these values to a channel instead of calling the
		// callback directly
		sb.OnCommitmentAddedToMempool(sector.sealed, msgCid, nil)
	}()
}

// NewBin implements binpack.Binner.
func (sb *SectorBuilder) NewBin() (binpack.Bin, error) {
	return sb.NewSector()
}

// BinSize implements binpack.Binner.
func (sb *SectorBuilder) BinSize() binpack.Space {
	return binpack.Space(sb.sectorSize)
}

// ItemSize implements binpack.Binner.
func (sb *SectorBuilder) ItemSize(item binpack.Item) binpack.Space {
	return binpack.Space(item.(*PieceInfo).Size)
}

// SpaceAvailable implements binpack.Binner.
func (sb *SectorBuilder) SpaceAvailable(bin binpack.Bin) binpack.Space {
	return binpack.Space(bin.(*UnsealedSector).numBytesFree)
}

// End binpack.Binner implementation

// NewSector allocates and returns a new UnsealedSector with file initialized, along with any error.
func (sb *SectorBuilder) NewSector() (s *UnsealedSector, err error) {
	res, err := sb.sectorStore.NewStagingSectorAccess()
	if err != nil {
		return nil, errors.Wrap(err, "failed to dispense staging sector unsealedSectorAccess")
	}

	s = &UnsealedSector{
		numBytesUsed:         sb.sectorSize,
		numBytesFree:         sb.sectorSize,
		unsealedSectorAccess: res.SectorAccess,
	}

	s.unsealedSectorAccess = res.SectorAccess

	return s, nil
}

// NewSealedSector creates a new SealedSector. The new SealedSector is appended to the slice of sealed sectors managed
// by the SectorBuilder.
func (sb *SectorBuilder) NewSealedSector(commR [32]byte, commD [32]byte, proof [192]byte, label, sealedSectorAccess string, s *UnsealedSector) *SealedSector {
	ss := &SealedSector{
		commD:                commD,
		commR:                commR,
		proof:                proof,
		sealedSectorAccess:   sealedSectorAccess,
		unsealedSectorAccess: s.unsealedSectorAccess,
		pieces:               s.pieces,
		numBytes:             s.numBytesUsed,
	}

	sb.sealedSectorsLk.Lock()
	defer sb.sealedSectorsLk.Unlock()
	sb.sealedSectors = append(sb.sealedSectors, ss)

	return ss
}

// InitSectorBuilder creates a new sector builder for the given miner. If a SectorBuilder had previously been created
// for the given miner, we reconstruct it using metadata from the datastore so that the miner can resume its work where
// it left off.
func InitSectorBuilder(nd *Node, minerAddr address.Address, sectorSize int, fs SectorDirs) (*SectorBuilder, error) {
	store := &sectorMetadataStore{
		store: nd.Repo.Datastore(),
	}

	sb := &SectorBuilder{
		dserv:         dag.NewDAGService(nd.Blockservice),
		MinerAddr:     minerAddr,
		nd:            nd,
		sectorSize:    uint64(sectorSize),
		metadataStore: store,
		sectorStore:   proofs.NewDiskBackedSectorStore(fs.StagingDir(), fs.SealedDir()),
	}

	sb.OnCommitmentAddedToMempool = sb.onCommitmentAddedToMempool
	metadata, err := store.getSectorBuilderMetadata(minerAddr)
	if err == nil {
		if err := configureSectorBuilderFromMetadata(store, sb, metadata); err != nil {
			return nil, err
		}

		sb.curUnsealedSectorLk.RLock()
		defer sb.curUnsealedSectorLk.RUnlock()

		return sb, sb.checkpoint(sb.curUnsealedSector)
	} else if strings.Contains(err.Error(), "not found") {
		if err := configureFreshSectorBuilder(sb); err != nil {
			return nil, err
		}
		sb.curUnsealedSectorLk.RLock()
		defer sb.curUnsealedSectorLk.RUnlock()

		return sb, sb.checkpoint(sb.curUnsealedSector)
	} else {
		return nil, err
	}
}

func configureSectorBuilderFromMetadata(store *sectorMetadataStore, sb *SectorBuilder, metadata *SectorBuilderMetadata) (finalErr error) {
	sector, err := store.getSector(metadata.CurUnsealedSectorAccess)
	if err != nil {
		return err
	}

	if err := sb.SyncFile(sector); err != nil {
		return errors.Wrapf(err, "failed to sync sector object with unsealed sector %s", sector.unsealedSectorAccess)
	}

	sb.curUnsealedSectorLk.Lock()
	sb.curUnsealedSector = sector
	sb.curUnsealedSectorLk.Unlock()

	for _, commR := range metadata.SealedSectorReplicaCommitments {
		sealed, err := store.getSealedSector(commR)
		if err != nil {
			return err
		}

		sb.sealedSectorsLk.Lock()
		sb.sealedSectors = append(sb.sealedSectors, sealed)
		sb.sealedSectorsLk.Unlock()
	}

	np := &binpack.NaivePacker{}
	np.InitWithCurrentBin(sb)
	sb.Packer = np

	return nil
}

func configureFreshSectorBuilder(sb *SectorBuilder) error {
	packer, firstBin, err := binpack.NewNaivePacker(sb)
	if err != nil {
		return err
	}
	sb.curUnsealedSectorLk.Lock()
	defer sb.curUnsealedSectorLk.Unlock()

	sb.curUnsealedSector = firstBin.(*UnsealedSector)
	sb.Packer = packer

	return nil
}

func (sb *SectorBuilder) onCommitmentAddedToMempool(*SealedSector, *cid.Cid, error) {
	// TODO: wait for commitSector message to be included in a block so that we
	// can update sealed sector metadata with the miner-created SectorID
}

// AddPiece writes the given piece into a sector
func (sb *SectorBuilder) AddPiece(ctx context.Context, pi *PieceInfo) (err error) {
	ctx = log.Start(ctx, "SectorBuilder.AddPiece")
	log.SetTag(ctx, "piece", pi)
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	bin, err := sb.Packer.AddItem(ctx, pi)
	if err == binpack.ErrItemTooLarge {
		return ErrPieceTooLarge
	}

	// If, during piece-writing, a greater-than-zero-amount of piece-bytes were
	// written to the unsealed sector file and we were unable to revert to the
	// pre-write state, ErrCouldNotRevertUnsealedSector will be returned. If we
	// were unable to revert, it is likely that the sector object and backing
	// unsealed sector-file are now in different states.
	if _, ok := err.(*ErrCouldNotRevertUnsealedSector); ok {
		panic(err)
	}

	if err == nil {
		if bin == nil {
			// What does this signify? Could use to signal something.
			panic("no bin returned from Packer.AddItem")
		}
		sb.curUnsealedSectorLk.Lock()
		sb.curUnsealedSector = bin.(*UnsealedSector)
		sb.curUnsealedSectorLk.Unlock()
	}

	sb.curUnsealedSectorLk.RLock()
	defer sb.curUnsealedSectorLk.RUnlock()

	// checkpoint after we've added the piece and updated the sector builder's
	// "current sector"
	if err := sb.checkpoint(sb.curUnsealedSector); err != nil {
		return err
	}

	return err
}

// SyncFile synchronizes the sector object and backing unsealed sector-file. SyncFile may mutate both the file and the
// sector object in order to achieve a consistent view of the sector.
// TODO: We should probably move most of this logic into Rust.
func (sb *SectorBuilder) SyncFile(s *UnsealedSector) error {
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
		storSize := s.numBytesUsed - s.numBytesFree

		if storSize == fileSize {
			return 0
		}
		if storSize < fileSize {
			return -1
		}
		return +1
	}

	// remove pieces from the sector until (s.numBytesUsed-s.numBytesFree) <= fi.size()
	for i := len(s.pieces) - 1; i >= 0; i-- {
		if cmpSize(s, res.NumBytes) == 1 {
			s.numBytesFree += s.pieces[i].Size
			s.pieces = s.pieces[:len(s.pieces)-1]
		} else {
			break
		}
	}

	if cmpSize(s, res.NumBytes) == -1 {
		return sb.sectorStore.TruncateUnsealed(proofs.TruncateUnsealedRequest{
			SectorAccess: s.unsealedSectorAccess,
			NumBytes:     s.numBytesUsed - s.numBytesFree,
		})
	}

	return nil
}

// SealAndAddCommitmentToMempool seals the given sector, adds the sealed sector's commitment to the message pool, and
// then returns the CID of the commitment message.
func (sb *SectorBuilder) SealAndAddCommitmentToMempool(ctx context.Context, s *UnsealedSector) (c *cid.Cid, err error) {
	ctx = log.Start(ctx, "SectorBuilder.SealAndAddCommitmentToMempool")
	log.SetTags(ctx, map[string]interface{}{
		"unsealedSectorAccess": s.unsealedSectorAccess,
	})
	defer func() {
		if c != nil {
			log.SetTag(ctx, "sectorCid", c.String())
		}
		log.FinishWithErr(ctx, err)
	}()

	ss, err := sb.Seal(ctx, s, sb.MinerAddr)
	if err != nil {
		// Hard to say what to do in this case.
		// Depending on the error, it could be "try again"
		// or 'verify data integrity and try again'
		return nil, errors.Wrap(err, "failed to seal sector")
	}

	s.sealed = ss
	if err := sb.checkpoint(s); err != nil {
		return nil, errors.Wrap(err, "failed to create checkpoint")
	}

	msgCid, err := sb.AddCommitmentToMempool(ctx, ss)
	if err != nil {
		// 'try again'
		// This can fail if the miners owner doesnt have enough funds to pay gas.
		// It can also happen if the miner included a deal in this sector that
		// is already sealed in a different sector.
		return nil, errors.Wrap(err, "failed to seal and add sector commitment")
	}

	return msgCid, nil
}

// AddCommitmentToMempool adds the sealed sector's commitment to the message pool and returns the CID of the commitment
// message.
func (sb *SectorBuilder) AddCommitmentToMempool(ctx context.Context, ss *SealedSector) (c *cid.Cid, err error) {
	ctx = log.Start(ctx, "SectorBuilder.AddCommitmentToMempool")
	log.SetTags(ctx, map[string]interface{}{
		"sealedSectorAccess": ss.sealedSectorAccess,
	})
	defer func() {
		if c != nil {
			log.SetTag(ctx, "sectorCid", c.String())
		}
		log.FinishWithErr(ctx, err)
	}()

	var deals []uint64
	for _, p := range ss.pieces {
		deals = append(deals, p.DealID)
	}

	args, err := abi.ToEncodedValues(
		noSectorID, // NB: we might already know the sector ID from having created it already
		ss.commR[:],
		deals,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to ABI encode commitSector arguments")
	}

	res, exitCode, err := sb.nd.CallQueryMethod(ctx, sb.MinerAddr, "getOwner", nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get miner owner")
	}

	if exitCode != uint8(0) {
		return nil, fmt.Errorf("failed to get miner owner, exitCode: %d", exitCode)
	}

	minerOwner, err := address.NewFromBytes(res[0])
	if err != nil {
		return nil, errors.Wrap(err, "received invalid mining owner")
	}
	msg := types.NewMessage(minerOwner, sb.MinerAddr, 0, nil, "commitSector", args)

	smsg, err := types.NewSignedMessage(*msg, sb.nd.Wallet)
	if err != nil {
		return nil, err
	}

	if err := sb.nd.AddNewMessage(ctx, smsg); err != nil {
		return nil, errors.Wrap(err, "pushing out commitSector message failed")
	}

	smsgCid, err := smsg.Cid()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get commitSector message CID")
	}

	return smsgCid, nil
}

// WritePiece writes data from the given reader to the sectors underlying storage
func (sb *SectorBuilder) WritePiece(ctx context.Context, s *UnsealedSector, pi *PieceInfo, r io.Reader) (finalErr error) {
	ctx = log.Start(ctx, "UnsealedSector.WritePiece")
	defer func() {
		log.FinishWithErr(ctx, finalErr)
	}()

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
		SectorAccess: s.unsealedSectorAccess,
		Data:         b,
	})
	if err != nil {
		return errors.Wrapf(err, "failed to write bytes to unsealed sector %s", s.unsealedSectorAccess)
	}

	if res.NumBytesWritten != pi.Size {
		err := fmt.Errorf("did not write all piece-bytes to file (pi.size=%d, wrote=%d)", pi.Size, n)

		if err1 := sb.SyncFile(s); err1 != nil {
			return NewErrCouldNotRevertUnsealedSector(err1, err)
		}

		return err
	}

	s.numBytesFree -= pi.Size
	s.pieces = append(s.pieces, pi)

	return nil
}

// Seal generates and returns a proof of replication along with supporting data.
func (sb *SectorBuilder) Seal(ctx context.Context, s *UnsealedSector, minerAddr address.Address) (_ *SealedSector, finalErr error) {
	ctx = log.Start(ctx, "SectorBuilder.Seal")
	defer func() {
		log.FinishWithErr(ctx, finalErr)
	}()

	res1, err := sb.sectorStore.NewSealedSectorAccess()
	if err != nil {
		return nil, errors.Wrap(err, "failed to dispense sealed sector unsealedSectorAccess")
	}

	proverID, err := proverID(minerAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create prover id from miner address")
	}

	req := proofs.SealRequest{
		ProverID:     proverID,
		SealedPath:   res1.SectorAccess,
		SectorID:     [31]byte{},
		Storage:      sb.sectorStore,
		UnsealedPath: s.unsealedSectorAccess,
	}

	res2, err := (&proofs.RustProver{}).Seal(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to seal sector")
	}

	return sb.NewSealedSector(res2.CommR, res2.CommD, res2.Proof, res1.SectorAccess, res1.SectorAccess, s), nil
}

// proverID creates a prover id by padding an address hash to 31 bytes
func proverID(addr address.Address) ([31]byte, error) {
	hash := addr.Hash()

	dlen := 31          // desired length
	hlen := len(hash)   // hash length
	padl := dlen - hlen // padding length

	var prid [31]byte

	// will copy dlen bytes from hash
	copy(prid[:], hash)

	if padl > 0 {
		copy(prid[hlen:], bytes.Repeat([]byte{0}, padl))
	}

	return prid, nil
}

func commRString(merkleRoot [32]byte) string {
	return base32.StdEncoding.EncodeToString(merkleRoot[:])
}
