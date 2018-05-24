package node

// TODO: mmap details are commented out but retained for future use.

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"path"
	"strings"
	"syscall"

	dag "gx/ipfs/QmNUCLv5fmUBuAcwbkt58NQvMcJgd5FPCYV2yNCXq4Wnd6/go-ipfs/merkledag"
	uio "gx/ipfs/QmNUCLv5fmUBuAcwbkt58NQvMcJgd5FPCYV2yNCXq4Wnd6/go-ipfs/unixfs/io"
	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	ds "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"

	drg "github.com/filecoin-project/go-proofs/porep/drgporep"
	"github.com/filecoin-project/go-proofs/porep/drgporep/drgraph"
	mmap "golang.org/x/sys/unix"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/util/binpack"
)

func init() {
	cbor.RegisterCborType(PieceInfo{})
}

// ErrPieceTooLarge is an error indicating that a piece cannot be larger than the sector into which it is written.
var ErrPieceTooLarge = errors.New("piece too large for sector")

const sectorSize = 1024
const nodeSize = 16

var noSectorID = big.NewInt(-1)

// SectorDirs describes the methods required to supply sector directories to a SectorBuilder.
type SectorDirs interface {
	StagingDir() string
	SealedDir() string
}

// PieceInfo is information about a filecoin piece
type PieceInfo struct {
	Ref    *cid.Cid
	Size   uint64
	DealID uint64
}

// SectorBuilder manages packing deals into sectors
// maybe this belongs somewhere else as part of a different thing?
type SectorBuilder struct {
	MinerAddr types.Address

	// TODO: information about all sectors needs to be persisted to disk
	CurSector        *Sector
	SealedSectors    []*SealedSector
	G                *drgraph.Graph
	Prover           *drg.Prover
	Packer           binpack.Packer
	publicParameters *PublicParameters

	stagingDir string
	sealedDir  string

	// yada yada don't hold a reference to this here, just take what you need

	nd    *Node
	store ds.Datastore
	dserv ipld.DAGService

	sectorSize uint64

	setup func([]byte, *PublicParameters, []byte) ([]byte, error)
}

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

	Label         string
	filename      string
	file          *os.File
	data          []byte
	sealed        *SealedSector
	sectorBuilder *SectorBuilder
	mmappedData   []byte
}

// SealedSector is a sector that has been sealed by the PoRep setup process
type SealedSector struct {
	merkleRoot []byte
	baseSector *Sector

	label string

	filename string
}

// Implement binpack.Binner

// AddItem implements binpack.Binner.
func (sb *SectorBuilder) AddItem(ctx context.Context, item binpack.Item, bin binpack.Bin) error {
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

	if err := s.WritePiece(pi, r); err != nil {
		return err
	}

	return nil
}

// CloseBin implements binpack.Binner.
func (sb *SectorBuilder) CloseBin(bin binpack.Bin) {
	go func() {
		err := sb.SealAndPostSector(context.Background(), bin.(*Sector))
		if err != nil {
			panic(err)
		}
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
	p, label := sb.newSectorPath()
	s = &Sector{
		Size:          sb.sectorSize,
		Free:          sb.sectorSize,
		Label:         label,
		sectorBuilder: sb,
	}
	err = os.MkdirAll(path.Dir(p), os.ModePerm)
	if err != nil {
		return s, err
	}
	s.filename = p
	s.file, err = os.Create(p)
	if err != nil {
		return s, err
	}
	return s, s.file.Close()
}

// NewSealedSector allocates and returns a new SealedSector from replicaData, merkleRoot, and baseSector, along with any error.
func (sb *SectorBuilder) NewSealedSector(merkleRoot []byte, baseSector *Sector) (ss *SealedSector,
	err error) {
	p, label := sb.newSealedSectorPath()
	ss = &SealedSector{
		merkleRoot: merkleRoot,
		baseSector: baseSector,
		label:      label,
	}

	sb.SealedSectors = append(sb.SealedSectors, ss)
	err = os.MkdirAll(path.Dir(p), os.ModePerm)
	if err != nil {
		return nil, err
	}

	if err := mmap.Munmap(baseSector.mmappedData); err != nil {
		return nil, err
	}

	if err := baseSector.file.Close(); err != nil {
		return nil, err
	}

	// move the now-sealed file out of staging
	err = os.Rename(baseSector.filename, p)
	if err != nil {
		return nil, err
	}

	ss.filename = p

	return ss, nil
}

// NewSectorBuilder creates a new sector builder from the given node
func NewSectorBuilder(nd *Node, sectorSize int, fs SectorDirs) (*SectorBuilder, error) {
	g := drgraph.NewDRSample(sectorSize / nodeSize)

	sb := &SectorBuilder{
		G:                g,
		Prover:           drg.NewProver("cats", g, nodeSize),
		publicParameters: filecoinParameters,
		nd:               nd,
		sectorSize:       uint64(sectorSize),
		store:            nd.Repo.Datastore(),
		dserv:            dag.NewDAGService(nd.Blockservice),
		setup:            proverSetup,
	}

	sb.stagingDir = fs.StagingDir()
	sb.sealedDir = fs.SealedDir()

	s, err := sb.NewSector()
	if err != nil {
		return nil, err
	}
	sb.CurSector = s

	packer, firstBin, err := binpack.NewNaivePacker(sb)
	if err != nil {
		return nil, err
	}
	sb.CurSector = firstBin.(*Sector)
	sb.Packer = packer

	return sb, sb.checkpoint()
}

// AddPiece writes the given piece into a sector
func (sb *SectorBuilder) AddPiece(ctx context.Context, pi *PieceInfo) error {
	bin, err := sb.Packer.AddItem(ctx, pi)
	if err == binpack.ErrItemTooLarge {
		return ErrPieceTooLarge
	}
	if err == nil {
		if bin == nil {
			// What does this signify? Could use to signal something.
			panic("no bin returned from Packer.AddItem")
		}
		sb.CurSector = bin.(*Sector)
	}

	return err
}

// These will be loaded from disk.
var filecoinParameters = makeFilecoinParameters(sectorSize, nodeSize)

// Create filecoin parameters (mainly used for testing).
func makeFilecoinParameters(sectorSize int, nodeSize int) *PublicParameters {
	return &PublicParameters{
		graph:     drgraph.NewDRSample(sectorSize / nodeSize),
		blockSize: uint64(nodeSize),
	}
}

// OpenAppend opens and sets sector's file for appending, returning the *os.file.
func (s *Sector) OpenAppend() error {
	f, err := os.OpenFile(s.filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	s.file = f
	return nil
}

// OpenMMap opens and sets sector's file for mmap
func (s *Sector) OpenMmap() error {
	var err error
	if err := os.Truncate(s.filename, int64(s.Size)); err != nil {
		errors.Wrap(err, "failed to truncate sector file")
	}
	if err != nil {
		return err
	}
	s.file, err = os.OpenFile(s.filename, os.O_RDWR, 0600)
	fd := s.file.Fd()
	offset := 0

	data, err := mmap.Mmap(int(fd), int64(offset), int(s.Size), syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED)
	if err != nil {
		// panic?
		return err
	}

	s.mmappedData = data

	return nil
}

// SealAndPostSector seals the given sector and posts its commitment on-chain
func (sb *SectorBuilder) SealAndPostSector(ctx context.Context, s *Sector) (err error) {
	ss, err := sb.Seal(s, sb.MinerAddr, sb.publicParameters)
	if err != nil {
		// Hard to say what to do in this case.
		// Depending on the error, it could be "try again"
		// or 'verify data integrity and try again'
		return
	}
	if err := sb.checkpointSealedMeta(ss); err != nil {
		return err
	}
	s.sealed = ss
	if err := sb.checkpointSectorMeta(s); err != nil {
		return err
	}
	if err := sb.PostSealedSector(ctx, ss); err != nil {
		// 'try again'
		// This can fail if the miners owner doesnt have enough funds to pay gas.
		// It can also happen if the miner included a deal in this sector that
		// is already sealed in a different sector.
		return err
	}
	return nil
}

// PostSealedSector posts the given sealed sector's commitment to the chain
func (sb *SectorBuilder) PostSealedSector(ctx context.Context, ss *SealedSector) error {
	var deals []uint64
	for _, p := range ss.baseSector.Pieces {
		deals = append(deals, p.DealID)
	}

	params, err := abi.ToEncodedValues(
		noSectorID, // NB: we might already know the sector ID from having created it already
		ss.merkleRoot,
		deals,
	)
	if err != nil {
		return err
	}

	minerOwner := types.Address{} // TODO: get the miner owner to send this from
	msg := types.NewMessage(minerOwner, sb.MinerAddr, 0, nil, "commitSector", params)

	if err := sb.nd.AddNewMessage(ctx, msg); err != nil {
		return errors.Wrap(err, "pushing out commitSector message failed")
	}

	// TODO: maybe wait for the message to clear on chain?
	return nil
}

// WritePiece writes data from the given reader to the sectors underlying storage
func (s *Sector) WritePiece(pi *PieceInfo, r io.Reader) (finalErr error) {
	if err := s.OpenAppend(); err != nil {
		return err
	}
	// Capture any error from the deferred close as the value to return unless previous error was being returned.
	defer func() {
		err := s.file.Close()
		if err != nil && finalErr == nil {
			finalErr = err
		}
	}()

	n, err := io.Copy(s.file, r)
	if err != nil {
		// TODO: make sure we roll back the state of the file to what it was before we started this write
		// Note, this should be a fault. We likely should already have all the
		// data locally before calling this method. If that ends up being the
		// case, this error signifies disk errors of some kind
		return err
	}

	// TODO:
	// We should be writing this out to a file in the 'staging' area. Once the
	// sector is ready to be sealed, we should mmap it and pass it to the
	// sealing code. The sealing code prefers fairly random access to the data,
	// so using mmap will be the fastest option. We could also provide an
	// interface to just do seeks and reads on the underlying file, but it
	// won't be as optimized

	if uint64(n) != pi.Size {
		// TODO: make sure we roll back the state of the file to what it was before we started this write
		return fmt.Errorf("reported piece size does not match what we read")
	}

	s.Free -= pi.Size
	s.Pieces = append(s.Pieces, pi)

	err = s.sectorBuilder.checkpointSectorMeta(s) // TODO: make this a method of Sector.
	return err
}

// PublicParameters is
type PublicParameters struct {
	graph     *drgraph.Graph
	blockSize uint64
}

func proverSetup(minerKey []byte, publicParams *PublicParameters, data []byte) ([]byte, error) {
	prover := drg.NewProver(string(minerKey), publicParams.graph, int(publicParams.blockSize))
	root, err := prover.Setup(NewMemoryReadWriter(data))

	if err != nil {
		return nil, errors.Wrap(err, "prover setup failed")
	}

	return root, nil
}

// Seal runs the PoRep setup process using the given parameters
// address may turn into a secret key
func (sb *SectorBuilder) Seal(s *Sector, minerID types.Address, publicParams *PublicParameters) (_ *SealedSector,
	finalErr error) {
	if err := s.OpenMmap(); err != nil {
		return nil, err
	}

	// AES key must be 16, 24, or 32 bytes. Failure to pad here breaks sealing.
	// TODO how should this key be derived?
	minerKey := make([]byte, 32)
	copy(minerKey[:len(minerID)], minerID[:])

	root, err := sb.setup(minerKey, publicParams, s.mmappedData)
	if err != nil {
		return nil, err
	}

	var copiedRoot = make([]byte, len(root))
	copy(copiedRoot, root)

	return sb.NewSealedSector(copiedRoot, s)
}

// ReadFile reads the content of a Sector's file into a byte array and returns it along with any error.
func (s *Sector) ReadFile() ([]byte, error) {
	return ioutil.ReadFile(s.filename)
}

// ReadFile reads the content of a SealedSector's file into a byte array and returns it along with any error.
func (ss *SealedSector) ReadFile() ([]byte, error) {
	return ioutil.ReadFile(ss.filename)
}

// newSectorLabel returns a newly-generated, random 32-character base32 string.
func newSectorLabel() string {
	c := 20
	b := make([]byte, c)
	_, err := rand.Read(b)
	if err != nil {
		panic("wtf")
	}
	encoded := base32.StdEncoding.EncodeToString(b)
	return encoded
}

// newSectorPath returns a path to a new (random) filename in the staging directory.
func (sb *SectorBuilder) newSectorPath() (pathname string, label string) {
	label = newSectorLabel()
	pathname = path.Join(sb.stagingDir, label)
	return pathname, label
}

// newSealedSectorPath returns a path to a new (random) filename in the sealed directory.
func (sb *SectorBuilder) newSealedSectorPath() (pathname string, label string) {
	label = newSectorLabel()
	pathname = path.Join(sb.sealedDir, label)
	return pathname, label
}

// merkleFilepath returns a path in smc's sealed directory, with name derived from ss's merkleRoot.
func (sb *SectorBuilder) merkleFilepath(ss *SealedSector) (string, error) {
	if ss.merkleRoot == nil {
		return "", errors.New("no merkleRoot")
	}
	merkleString := merkleString(ss.merkleRoot)
	merkleString = strings.Trim(merkleString, "=")

	return path.Join(sb.sealedDir, merkleString), nil
}

func merkleString(merkleRoot []byte) string {
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
