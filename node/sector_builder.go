package node

import (
	"bytes"
	"context"
	"encoding/base32"
	"fmt"
	"io"
	"math/big"
	"os"
	"path"
	"strings"
	"sync"

	cbor "gx/ipfs/QmV6BQ6fFCf9eFHDuRxvguvqfKLZtZrxthgZvDfRCs4tMN/go-ipld-cbor"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	ipld "gx/ipfs/QmX5CsuHyVZeTLxgRSYkgLSDQKb9UjE8xnhQzCEJWWWFsC/go-ipld-format"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	uio "gx/ipfs/Qmdg2crJzNUF1mLPnLPSCCaDdLDqE4Qrh9QEiDooSYkvuB/go-unixfs/io"
	dag "gx/ipfs/QmeLG6jF1xvEmHca5Vy4q4EdQWp8Xq9S6EPyZrN9wvSRLC/go-merkledag"

	"github.com/filecoin-project/go-filecoin/abi"
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
	MinerAddr types.Address

	curSectorLk    sync.RWMutex // curSectorLk protects curSector
	curSector      *Sector
	sealedSectorLk sync.RWMutex // sealedSectorLk protects sealedSectors
	sealedSectors  []*SealedSector

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

// Sector is a filecoin storage sector. A miner fills this up with data, and
// then seals it. The first part I can do, the second part needs to be figured out more.
// Somehow, I turn this piecemap and backing data buffer into something that the chain can verify.
// So something X has to be computed from this that convinces the chain that
// this miner is storing all the deals referenced in the piece map...
type Sector struct {
	Size   uint64
	Free   uint64
	Pieces []*PieceInfo
	ID     int64

	Label    string // TODO: replace Label and filename with SectorAccess
	filename string // TODO: replace Label and filename with SectorAccess
	sealed   *SealedSector
}

var _ binpack.Bin = &Sector{}

// SealedSector is a sector that has been sealed by the PoRep setup process
type SealedSector struct {
	commD       []byte
	commR       []byte
	snarkProof  []byte
	filename    string // TODO: replace label and filename with SectorAccess
	label       string
	sectorLabel string // TODO: replace with sectorFileAccess
	pieces      []*PieceInfo
	size        uint64
}

// GetCurrentBin implements Binner.
func (sb *SectorBuilder) GetCurrentBin() binpack.Bin {
	sb.curSectorLk.RLock()
	defer sb.curSectorLk.RUnlock()

	return sb.curSector
}

// AddItem implements binpack.Binner.
func (sb *SectorBuilder) AddItem(ctx context.Context, item binpack.Item, bin binpack.Bin) (err error) {
	ctx = log.Start(ctx, "SectorBuilder.AddItem")
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	pi := item.(*PieceInfo)
	s := bin.(*Sector)
	root, err := sb.dserv.Get(ctx, pi.Ref)
	if err != nil {
		return err
	}

	r, err := uio.NewDagReader(ctx, root, sb.dserv)
	if err != nil {
		return err
	}

	if err := s.WritePiece(ctx, pi, r); err != nil {
		return err
	}

	return nil
}

// CloseBin implements binpack.Binner.
func (sb *SectorBuilder) CloseBin(bin binpack.Bin) {
	go func() {
		sector := bin.(*Sector)
		msgCid, err := sb.SealAndAddCommitmentToMempool(context.Background(), sector)
		if err != nil {
			sb.OnCommitmentAddedToMempool(nil, nil, errors.Wrap(err, "failed to seal and commit sector"))
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
	return binpack.Space(bin.(*Sector).Free)
}

// End binpack.Binner implementation

// NewSector allocates and returns a new Sector with file initialized, along with any error.
func (sb *SectorBuilder) NewSector() (s *Sector, err error) {
	access, err := sb.sectorStore.NewStagingSectorAccess()
	if err != nil {
		return nil, errors.Wrap(err, "failed to dispense staging sector access")
	}

	s = &Sector{
		Size:  sb.sectorSize,
		Free:  sb.sectorSize,
		Label: access,
	}

	err = os.MkdirAll(path.Dir(access), os.ModePerm)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create sector directory")
	}
	s.filename = access

	return s, nil
}

// NewSealedSector creates a new SealedSector. The new SealedSector is appended to the slice of sealed sectors managed
// by the SectorBuilder.
func (sb *SectorBuilder) NewSealedSector(commR []byte, commD []byte, snarkProof []byte, label, path string, s *Sector) *SealedSector {
	s.filename = ""

	ss := &SealedSector{
		commD:       commD,
		commR:       commR,
		snarkProof:  snarkProof,
		filename:    path,
		label:       label,
		sectorLabel: s.Label,
		pieces:      s.Pieces,
		size:        s.Size,
	}

	sb.sealedSectorLk.Lock()
	defer sb.sealedSectorLk.Unlock()
	sb.sealedSectors = append(sb.sealedSectors, ss)

	return ss
}

// InitSectorBuilder creates a new sector builder for the given miner. If a SectorBuilder had previously been created
// for the given miner, we reconstruct it using metadata from the datastore so that the miner can resume its work where
// it left off.
func InitSectorBuilder(nd *Node, minerAddr types.Address, sectorSize int, fs SectorDirs) (*SectorBuilder, error) {
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

		sb.curSectorLk.RLock()
		defer sb.curSectorLk.RUnlock()

		return sb, sb.checkpoint(sb.curSector)
	} else if strings.Contains(err.Error(), "not found") {
		if err := configureFreshSectorBuilder(sb); err != nil {
			return nil, err
		}
		sb.curSectorLk.RLock()
		defer sb.curSectorLk.RUnlock()

		return sb, sb.checkpoint(sb.curSector)
	} else {
		return nil, err
	}
}

func configureSectorBuilderFromMetadata(store *sectorMetadataStore, sb *SectorBuilder, metadata *SectorBuilderMetadata) (finalErr error) {
	sector, err := store.getSector(metadata.CurSectorLabel)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(sector.filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}

	defer func() {
		if err := f.Close(); err != nil && finalErr == nil {
			finalErr = errors.Wrapf(err, "failed to close unsealed sector-file %s", sector.filename)
		}
	}()

	if err := sector.SyncFile(f); err != nil {
		return errors.Wrapf(err, "failed to sync sector object with unsealed sector-file %s", sector.filename)
	}

	sb.curSectorLk.Lock()
	sb.curSector = sector
	sb.curSectorLk.Unlock()

	for _, commR := range metadata.SealedSectorReplicaCommitments {
		sealed, err := store.getSealedSector(commR)
		if err != nil {
			return err
		}

		sb.sealedSectorLk.Lock()
		sb.sealedSectors = append(sb.sealedSectors, sealed)
		sb.sealedSectorLk.Unlock()
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
	sb.curSectorLk.Lock()
	defer sb.curSectorLk.Unlock()

	sb.curSector = firstBin.(*Sector)
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
		sb.curSectorLk.Lock()
		sb.curSector = bin.(*Sector)
		sb.curSectorLk.Unlock()
	}

	sb.curSectorLk.RLock()
	defer sb.curSectorLk.RUnlock()

	// checkpoint after we've added the piece and updated the sector builder's
	// "current sector"
	if err := sb.checkpoint(sb.curSector); err != nil {
		return err
	}

	return err
}

// SyncFile synchronizes the sector object and backing unsealed sector-file. SyncFile may mutate both the file and the
// sector object in order to achieve a consistent view of the sector.
func (s *Sector) SyncFile(f *os.File) error {
	fi, err := f.Stat()
	if err != nil {
		return errors.Wrapf(err, "failed to stat sector file %s", s.filename)
	}

	// | file size | metadata pieces | action                                        |
	// |-----------|-----------------|-----------------------------------------------|
	// | 500 bytes | 100|100|100     | truncate file to 300 bytes                    |
	// | 500 bytes | 100|100|400     | truncate file to 200 bytes, pieces to 100|100 |
	// | 500 bytes | 100|400         | noop                                          |

	cmpSize := func(s *Sector, info os.FileInfo) int {
		storSize := int64(s.Size - s.Free)
		fileSize := info.Size()

		if storSize == fileSize {
			return 0
		}
		if storSize < fileSize {
			return -1
		}
		return +1
	}

	// remove pieces from the sector until (s.Size-s.Free) <= fi.Size()
	for i := len(s.Pieces) - 1; i >= 0; i-- {
		if cmpSize(s, fi) == 1 {
			s.Free += s.Pieces[i].Size
			s.Pieces = s.Pieces[:len(s.Pieces)-1]
		} else {
			break
		}
	}

	if cmpSize(s, fi) == -1 {
		return f.Truncate(int64(s.Size - s.Free))
	}

	return nil
}

// SealAndAddCommitmentToMempool seals the given sector, adds the sealed sector's commitment to the message pool, and
// then returns the CID of the commitment message.
func (sb *SectorBuilder) SealAndAddCommitmentToMempool(ctx context.Context, s *Sector) (c *cid.Cid, err error) {
	ctx = log.Start(ctx, "SectorBuilder.SealAndAddCommitmentToMempool")
	log.SetTags(ctx, map[string]interface{}{
		"fileName": s.filename,
		"lable":    s.Label,
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
		"fileName": ss.filename,
		"lable":    ss.label,
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
		ss.commR,
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

	minerOwner, err := types.NewAddressFromBytes(res[0])
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
func (s *Sector) WritePiece(ctx context.Context, pi *PieceInfo, r io.Reader) (finalErr error) {
	ctx = log.Start(ctx, "Sector.WritePiece")
	defer func() {
		log.FinishWithErr(ctx, finalErr)
	}()

	f, err := os.OpenFile(s.filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}

	defer func() {
		if err := f.Close(); err != nil && finalErr == nil {
			finalErr = errors.Wrapf(err, "failed to close unsealed sector-file %s", s.filename)
		}
	}()

	n, err := io.Copy(f, r)
	if err != nil {
		return errors.Wrapf(err, "failed to copy bytes to unsealed sector-file %s", s.filename)
	}

	if uint64(n) != pi.Size {
		err := fmt.Errorf("did not write all piece-bytes to file (pi.Size=%d, wrote=%d)", pi.Size, n)

		if err1 := s.SyncFile(f); err1 != nil {
			return NewErrCouldNotRevertUnsealedSector(err1, err)
		}

		return err
	}

	s.Free -= pi.Size
	s.Pieces = append(s.Pieces, pi)

	return nil
}

// Seal generates and returns a proof of replication along with supporting data.
func (sb *SectorBuilder) Seal(ctx context.Context, s *Sector, minerAddr types.Address) (_ *SealedSector, finalErr error) {
	ctx = log.Start(ctx, "SectorBuilder.Seal")
	defer func() {
		log.FinishWithErr(ctx, finalErr)
	}()

	access, err := sb.sectorStore.NewSealedSectorAccess()
	if err != nil {
		return nil, errors.Wrap(err, "failed to dispense sealed sector access")
	}

	if err := os.MkdirAll(path.Dir(access), os.ModePerm); err != nil {
		return nil, errors.Wrap(err, "failed to create sealed sector path")
	}

	proverID, err := proverID(minerAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create prover id from miner address")
	}

	req := proofs.SealRequest{
		UnsealedPath:  s.filename,
		SealedPath:    access,
		ChallengeSeed: make([]byte, 32), // TODO: derive from chain
		ProverID:      proverID,
		RandomSeed:    make([]byte, 32), // TODO: create real seed
	}

	res, err := (&proofs.RustProver{}).Seal(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to seal sector")
	}

	return sb.NewSealedSector(res.CommR, res.CommD, res.SnarkProof, access, access, s), nil
}

// proverID creates a prover id by padding an address hash to 31 bytes
func proverID(addr types.Address) ([]byte, error) {
	hash := addr.Hash()

	dlen := 31          // desired length
	hlen := len(hash)   // hash length
	padl := dlen - hlen // padding length

	prid := make([]byte, 31)

	// will copy dlen bytes from hash
	copy(prid, hash)

	if padl > 0 {
		copy(prid[hlen:], bytes.Repeat([]byte{0}, padl))
	}

	return prid, nil
}

func commRString(merkleRoot []byte) string {
	return base32.StdEncoding.EncodeToString(merkleRoot)
}

// MemoryReadWriter implements data.ReadWriter and represents a simple byte slice in memory.
type MemoryReadWriter struct {
	data []byte
}

// NewMemoryReadWriter returns MemoryReadWriter initialized with data.
func NewMemoryReadWriter(data []byte) MemoryReadWriter {
	return MemoryReadWriter{data: data}
}

// DataAt implements data.ReadWriter.
func (mrw MemoryReadWriter) DataAt(offset, length uint64, cb func([]byte) error) error {
	if length == 0 {
		// Define 0 length to mean read/write to end of data.
		return cb(mrw.data[offset:])
	}
	return cb(mrw.data[offset : offset+length])
}
