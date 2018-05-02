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
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/paper-porep-drg/go-porep/drg"
	"github.com/filecoin-project/paper-porep-drg/go-porep/drg/drgraph"
)

// ErrPieceTooLarge is an error indicating that a piece cannot be larger than the sector into which it is written.
var ErrPieceTooLarge = errors.New("piece too large for sector")

const sectorSize = 1024
const nodeSize = 16

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

	filename string
	file     *os.File
	data     []byte
	sealed   *SealedSector
	//ReaderAt *mmap.ReaderAt
}

// SectorBuilder manages packing deals into sectors
// maybe this belongs somewhere else as part of a different thing?
type SectorBuilder struct {
	MinerAddr types.Address

	// TODO: information about all sectors needs to be persisted to disk
	CurSector     *Sector
	SealedSectors []*SealedSector

	G      *drgraph.Graph
	Prover *drg.Prover

	stagingDir string
	sealedDir  string

	// yada yada don't hold a reference to this here, just take what you need
	nd         *Node
	sectorSize uint64
}

// NewSector allocates and returns a new Sector with file initialized, along with any error.
func (smc *SectorBuilder) NewSector() (s *Sector, err error) {
	s = &Sector{
		Size: smc.sectorSize,
		Free: smc.sectorSize,
	}
	p := smc.newSectorPath()
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
func (smc *SectorBuilder) NewSealedSector(replicaData []byte, merkleRoot []byte, baseSector *Sector) (ss *SealedSector,
	err error) {
	ss = &SealedSector{
		replicaData: replicaData,
		merkleRoot:  merkleRoot,
		baseSector:  baseSector,
	}
	p := smc.newSealedSectorPath()
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
	smc := &SectorBuilder{
		G:          g,
		Prover:     drg.NewProver("cats", g, nodeSize),
		nd:         nd,
		sectorSize: uint64(sectorSize),
	}
	smc.stagingDir = fs.StagingDir()
	smc.sealedDir = fs.SealedDir()

	s, err := smc.NewSector()
	if err != nil {
		return nil, err
	}
	smc.CurSector = s
	return smc, nil
}

// AddPiece writes the given piece into a sector
func (smc *SectorBuilder) AddPiece(ctx context.Context, pi *PieceInfo) error {
	if pi.Size > smc.CurSector.Size {
		return ErrPieceTooLarge
	}
	if smc.CurSector.Free < pi.Size {
		// No room for piece in current sector.
		curSector := smc.CurSector
		newSector, err := smc.NewSector()
		if err != nil {
			return err
		}
		smc.CurSector = newSector

		go smc.SealAndPostSector(context.Background(), curSector)
	}

	dserv := dag.NewDAGService(smc.nd.Blockservice)
	root, err := dserv.Get(ctx, pi.Ref)
	if err != nil {
		return err
	}

	r, err := uio.NewDagReader(ctx, root, dserv)
	if err != nil {
		return err
	}

	if err := smc.CurSector.WritePiece(pi, r); err != nil {
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
func (smc *SectorBuilder) SealAndPostSector(ctx context.Context, s *Sector) {
	ss, err := smc.Seal(s, smc.MinerAddr, filecoinParameters)
	if err != nil {
		// Hard to say what to do in this case.
		// Depending on the error, it could be "try again"
		// or 'verify data integrity and try again'
		panic(err)
	}

	s.sealed = ss

	if err := smc.PostSealedSector(ctx, ss); err != nil {
		// 'try again'
		// This can fail if the miners owner doesnt have enough funds to pay gas.
		// It can also happen if the miner included a deal in this sector that
		// is already sealed in a different sector.
		panic(err)
	}
}

// PostSealedSector posts the given sealed sector's commitment to the chain
func (smc *SectorBuilder) PostSealedSector(ctx context.Context, ss *SealedSector) error {
	var deals []uint64
	for _, p := range ss.baseSector.Pieces {
		deals = append(deals, p.DealID)
	}

	params, err := abi.ToEncodedValues(
		big.NewInt(-1), // NB: we might already know the sector ID from having created it already
		ss.merkleRoot,
		deals,
	)
	if err != nil {
		return err
	}

	minerOwner := types.Address{} // TODO: get the miner owner to send this from
	msg := types.NewMessage(minerOwner, smc.MinerAddr, 0, nil, "commitSector", params)

	if err := smc.nd.AddNewMessage(ctx, msg); err != nil {
		return errors.Wrap(err, "pushing out commitSector message failed")
	}

	// TODO: maybe wait for the message to clear on chain?
	return nil
}

// WritePiece writes data from the given reader to the sectors underlying storage
func (s *Sector) WritePiece(pi *PieceInfo, r io.Reader) error {
	if err := s.OpenAppend(); err != nil {
		return err
	}

	n, err := io.Copy(s.file, r)
	if err != nil {
		// TODO: make sure we roll back the state of the file to what it was before we started this write
		// Note, this should be a fault. We likely should already have all the
		// data locally before calling this method. If that ends up being the
		// case, this error signifies disk errors of some kind
		return err
	}

	// TODO: this is temporary. Use the above code later
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

	return s.file.Close()
}

// PublicParameters is
type PublicParameters struct {
	graph     *drgraph.Graph
	blockSize uint64
}

// SealedSector is a sector that has been sealed by the PoRep setup process
type SealedSector struct {
	replicaData []byte
	merkleRoot  []byte
	baseSector  *Sector

	filename string
	file     *os.File
}

// Seal runs the PoRep setup process using the given parameters
// address may turn into a secret key
func (smc *SectorBuilder) Seal(s *Sector, minerID types.Address, publicParams *PublicParameters) (*SealedSector,
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

	ss, err := smc.NewSealedSector(prover.Replica, root, s)
	if err != nil {
		return nil, err
	}
	err = ss.Dump()
	if err != nil {
		return ss, err
	}
	merklePath, err := smc.merkleFilepath(ss)
	if err != nil {
		return ss, err
	}
	err = os.Rename(ss.filename, merklePath)
	if err == nil {
		ss.filename = merklePath
	}
	return ss, err
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

// newSectorFileName returns a newly-generated, random 32-character base32 string.
func newSectorFileName() string {
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
func (smc *SectorBuilder) newSectorPath() string {
	// FIXME: Lookup or create by ID in metadata -- Repo.Datastore.
	return path.Join(smc.stagingDir, newSectorFileName())
}

// newSealedSectorPath returns a path to a new (random) filename in the sealed directory.
func (smc *SectorBuilder) newSealedSectorPath() string {
	// FIXME: Lookup or create by ID in metadata -- Repo.Datastore.
	return path.Join(smc.sealedDir, newSectorFileName())
}

// merkleFilepath returns a path in smc's sealed directory, with name derived from ss's merkleRoot.
func (smc *SectorBuilder) merkleFilepath(ss *SealedSector) (string, error) {
	if ss.merkleRoot == nil {
		return "", errors.New("no merkleRoot")
	}
	merkleString := base32.StdEncoding.EncodeToString(ss.merkleRoot)
	merkleString = strings.Trim(merkleString, "=")

	return path.Join(smc.sealedDir, merkleString), nil
}
