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

	dag "gx/ipfs/QmNUCLv5fmUBuAcwbkt58NQvMcJgd5FPCYV2yNCXq4Wnd6/go-ipfs/merkledag"
	uio "gx/ipfs/QmNUCLv5fmUBuAcwbkt58NQvMcJgd5FPCYV2yNCXq4Wnd6/go-ipfs/unixfs/io"
	//	"golang.org/x/exp/mmap"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	ds "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/paper-porep-drg/go-porep/drg"
	"github.com/filecoin-project/paper-porep-drg/go-porep/drg/drgraph"
)

func init() {
	cbor.RegisterCborType(PieceInfo{})
	cbor.RegisterCborType(SectorMetadata{})
	cbor.RegisterCborType(SealedSectorMetadata{})
	cbor.RegisterCborType(SectorBuilderMetadata{})
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
	CurSector     *Sector
	SealedSectors []*SealedSector
	G             *drgraph.Graph
	Prover        *drg.Prover

	stagingDir string
	sealedDir  string

	// yada yada don't hold a reference to this here, just take what you need

	nd    *Node
	store ds.Datastore
	dserv ipld.DAGService

	sectorSize uint64
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
	//ReaderAt *mmap.ReaderAt
}

// SealedSector is a sector that has been sealed by the PoRep setup process
type SealedSector struct {
	replicaData []byte
	merkleRoot  []byte
	baseSector  *Sector

	label string

	filename string
	file     *os.File
}

// SectorMetadata represent the persistent metadata associated with a Sector.
type SectorMetadata struct {
	StagingPath string
	Pieces      []*PieceInfo
	Size        uint64
	Free        uint64
	MerkleRoot  []byte
}

// SealedSectorMetadata represent the persistent metadata associated with a SealedSector.
type SealedSectorMetadata struct {
	MerkleRoot      []byte
	Label           string
	BaseSectorLabel string
	SealedPath      string
}

// SectorBuilderMetadata represent the persistent metadata associated with a SectorBuilder.
type SectorBuilderMetadata struct {
	Miner         types.Address
	Sectors       []string
	SealedSectors []string
}

// SectorMetadata returns the metadata associated with a Sector.
func (s *Sector) SectorMetadata() *SectorMetadata {
	meta := &SectorMetadata{
		StagingPath: s.filename,
		Pieces:      s.Pieces,
		Size:        s.Size,
		Free:        s.Free,
		MerkleRoot:  []byte{},
	}
	if s.sealed != nil {
		meta.MerkleRoot = s.sealed.merkleRoot
	}
	return meta
}

// SealedSectorMetadata returns the metadata associated with a SealedSector.
func (ss *SealedSector) SealedSectorMetadata() *SealedSectorMetadata {
	meta := &SealedSectorMetadata{
		MerkleRoot:      ss.merkleRoot,
		Label:           ss.label,
		BaseSectorLabel: ss.baseSector.Label,
		SealedPath:      ss.filename,
	}
	return meta
}

// SectorBuilderMetadata returns the metadata associated with a SectorBuilderMetadata.
func (sb *SectorBuilder) SectorBuilderMetadata() *SectorBuilderMetadata {
	meta := SectorBuilderMetadata{
		Miner:         sb.MinerAddr,
		Sectors:       []string{sb.CurSector.Label},
		SealedSectors: make([]string, len(sb.SealedSectors)),
	}
	for i, sealed := range sb.SealedSectors {
		meta.SealedSectors[i] = sealed.label
	}
	return &meta
}

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
func (sb *SectorBuilder) NewSealedSector(replicaData []byte, merkleRoot []byte, baseSector *Sector) (ss *SealedSector,
	err error) {
	p, label := sb.newSealedSectorPath()
	ss = &SealedSector{
		replicaData: replicaData,
		merkleRoot:  merkleRoot,
		baseSector:  baseSector,
		label:       label,
	}
	sb.SealedSectors = append(sb.SealedSectors, ss)
	err = os.MkdirAll(path.Dir(p), os.ModePerm)
	if err != nil {
		return ss, err
	}
	ss.filename = p
	ss.file, err = os.Create(p)
	if err != nil {
		return ss, err
	}
	return ss, ss.file.Close()
}

// NewSectorBuilder creates a new sector builder from the given node
func NewSectorBuilder(nd *Node, sectorSize int, fs SectorDirs) (*SectorBuilder, error) {
	g := drgraph.New(sectorSize / nodeSize)
	sb := &SectorBuilder{
		G:          g,
		Prover:     drg.NewProver("cats", g, nodeSize),
		nd:         nd,
		sectorSize: uint64(sectorSize),
		store:      nd.Repo.Datastore(),
		dserv:      dag.NewDAGService(nd.Blockservice),
	}

	sb.stagingDir = fs.StagingDir()
	sb.sealedDir = fs.SealedDir()

	s, err := sb.NewSector()
	if err != nil {
		return nil, err
	}
	sb.CurSector = s
	return sb, sb.checkpoint()
}

func (sb *SectorBuilder) checkpoint() error {
	sector := sb.CurSector
	if err := sb.checkpointBuilderMeta(); err != nil {
		return err
	}
	if err := sb.checkpointSectorMeta(sector); err != nil {
		return err
	}
	if sector.sealed != nil {
		return sb.checkpointSealedMeta(sector.sealed)
	}
	return nil

}

// AddPiece writes the given piece into a sector
func (sb *SectorBuilder) AddPiece(ctx context.Context, pi *PieceInfo) error {
	if pi.Size > sb.CurSector.Size {
		return ErrPieceTooLarge
	}
	if sb.CurSector.Free < pi.Size {
		// No room for piece in current sector.
		oldSector := sb.CurSector
		newSector, err := sb.NewSector()
		if err != nil {
			return err
		}
		sb.CurSector = newSector

		go func() {
			err := sb.SealAndPostSector(context.Background(), oldSector)
			if err != nil {
				panic(err)
			}
		}()
	}

	root, err := sb.dserv.Get(ctx, pi.Ref)
	if err != nil {
		return err
	}

	r, err := uio.NewDagReader(ctx, root, sb.dserv)
	if err != nil {
		return err
	}

	if err := sb.CurSector.WritePiece(pi, r); err != nil {
		return err
	}

	return nil
}

var filecoinParameters = &PublicParameters{
	// this will be loaded from disk
	graph:     drgraph.New(sectorSize / nodeSize),
	blockSize: nodeSize,
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

// OpenMMap opens and sets sector's file for mmap, returning the *mmap.ReaderAt.
//func (s *Sector) OpenMMap() *mmap.ReaderAt {
//	r, err := mmap.Open(s.filename)
//	if err != nil {
//		panic("could not mmap sector")
//	}
//	s.ReaderAt = r
//	return r
//}

// SealAndPostSector seals the given sector and posts its commitment on-chain
func (sb *SectorBuilder) SealAndPostSector(ctx context.Context, s *Sector) (err error) {
	ss, err := sb.Seal(s, sb.MinerAddr, filecoinParameters)
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
	// Capture any error from the deferred close as the value to return.
	defer func() { finalErr = s.file.Close() }()

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
	if err != nil {
		return err
	}
	return nil
}

// PublicParameters is
type PublicParameters struct {
	graph     *drgraph.Graph
	blockSize uint64
}

// Seal runs the PoRep setup process using the given parameters
// address may turn into a secret key
func (sb *SectorBuilder) Seal(s *Sector, minerID types.Address, publicParams *PublicParameters) (*SealedSector,
	error) {
	// TODO: Actually use the mmap.ReaderAt for the sealing, instead of passing s.data.
	/* defer s.OpenMMap().Close() // nolint: errcheck

	if err := s.ReaderAt.Close(); err != nil {
		panic(err)
	}
	*/

	// TODO: Remove -- eventually we won't pass the data along, but for now it's necessary (and useful for testing).
	data, err := s.ReadFile()
	if err != nil {
		return nil, err
	}
	s.data = data

	if uint64(len(s.data)) < s.Size {
		s.data = append(s.data, make([]byte, s.Size-uint64(len(s.data)))...)
	}
	prover := drg.NewProver(string(minerID[:]), publicParams.graph, int(publicParams.blockSize))
	_, root := prover.Setup(s.data)

	ss, err := sb.NewSealedSector(prover.Replica, root, s)
	// NOTE: Between here and the call to os.Rename below, the SealedSector has a filename which is never persisted.
	// FIXME: Does it need to be?
	if err != nil {
		return nil, err
	}
	err = ss.Dump()
	if err != nil {
		return ss, err
	}
	merklePath, err := sb.merkleFilepath(ss)
	if err != nil {
		return ss, err
	}
	err = os.Rename(ss.filename, merklePath)
	if err == nil {
		ss.filename = merklePath
	} else {
		return ss, err
	}
	return ss, nil
}

// ReadFile reads the content of a Sector's file into a byte array and returns it along with any error.
func (s *Sector) ReadFile() ([]byte, error) {
	return ioutil.ReadFile(s.filename)
}

// ReadFile reads the content of a SealedSector's file into a byte array and returns it along with any error.
func (ss *SealedSector) ReadFile() ([]byte, error) {
	return ioutil.ReadFile(ss.filename)
}

// Dump dumps ss's replicaData to disk.
func (ss *SealedSector) Dump() error {
	return ioutil.WriteFile(ss.filename, ss.replicaData, 0600)
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

func (sb *SectorBuilder) metadataKey(label string) ds.Key {
	path := []string{"sectors", "metadata"}
	return ds.KeyWithNamespaces(path).Instance(label)
}

func (sb *SectorBuilder) sealedMetadataKey(merkleRoot []byte) ds.Key {
	path := []string{"sealedSectors", "metadata"}
	return ds.KeyWithNamespaces(path).Instance(merkleString(merkleRoot))
}

func (sb *SectorBuilder) builderMetadataKey(minerAddress types.Address) ds.Key {
	path := []string{"sectors", "metadata"}
	return ds.KeyWithNamespaces(path).Instance(minerAddress.String())
}

// GetMeta returns SectorMetadata for sector labeled, label, and any error.
func (sb *SectorBuilder) GetMeta(label string) (*SectorMetadata, error) {
	key := sb.metadataKey(label)

	data, err := sb.store.Get(key)
	if err != nil {
		return nil, err
	}
	var m SectorMetadata
	if err := cbor.DecodeInto(data.([]byte), &m); err != nil {
		return nil, err
	}
	return &m, err
}

// GetSealedMeta returns SealedSectorMetadata for merkleRoot, and any error.
func (sb *SectorBuilder) GetSealedMeta(merkleRoot []byte) (*SealedSectorMetadata, error) {
	key := sb.sealedMetadataKey(merkleRoot)

	data, err := sb.store.Get(key)
	if err != nil {
		return nil, err
	}
	var m SealedSectorMetadata
	if err := cbor.DecodeInto(data.([]byte), &m); err != nil {
		return nil, err
	}

	return &m, err
}

// GetBuilderMeta returns SectorBuilderMetadata for SectorBuilder, sb, and any error.
func (sb *SectorBuilder) GetBuilderMeta(minerAddress types.Address) (*SectorBuilderMetadata, error) {
	key := sb.builderMetadataKey(minerAddress)

	data, err := sb.store.Get(key)
	if err != nil {
		return nil, err
	}
	var m SectorBuilderMetadata
	if err := cbor.DecodeInto(data.([]byte), &m); err != nil {
		return nil, err
	}
	return &m, err
}

func (sb *SectorBuilder) setMeta(label string, meta *SectorMetadata) error {
	key := sb.metadataKey(label)
	data, err := cbor.DumpObject(meta)
	if err != nil {
		return err
	}
	return sb.store.Put(key, data)
}

func (sb *SectorBuilder) setSealedMeta(merkleRoot []byte, meta *SealedSectorMetadata) error {
	key := sb.sealedMetadataKey(merkleRoot)
	data, err := cbor.DumpObject(meta)
	if err != nil {
		return err
	}
	return sb.store.Put(key, data)
}

func (sb *SectorBuilder) setBuilderMeta(minerAddress types.Address, meta *SectorBuilderMetadata) error {
	key := sb.metadataKey(minerAddress.String())
	data, err := cbor.DumpObject(meta)
	if err != nil {
		return err
	}
	return sb.store.Put(key, data)
}

// TODO: only actually set if changed.
func (sb *SectorBuilder) checkpointSectorMeta(s *Sector) error {
	return sb.setMeta(s.Label, s.SectorMetadata())
}

func (sb *SectorBuilder) checkpointSealedMeta(ss *SealedSector) error {
	return sb.setSealedMeta(ss.merkleRoot, ss.SealedSectorMetadata())
}

func (sb *SectorBuilder) checkpointBuilderMeta() error {
	return sb.setBuilderMeta(sb.MinerAddr, sb.SectorBuilderMetadata())
}
