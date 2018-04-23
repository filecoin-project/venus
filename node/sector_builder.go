package node

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/paper-porep-drg/go-porep/drg"
	"github.com/filecoin-project/paper-porep-drg/go-porep/drg/drgraph"

	dag "github.com/ipfs/go-ipfs/merkledag"
	uio "github.com/ipfs/go-ipfs/unixfs/io"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

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

	//file     *os.File
	data []byte
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

	// yada yada don't hold a reference to this here, just take what you need
	nd *Node
}

const sectorSize = 1024
const nodeSize = 16

// NewSectorBuilder creates a new sector builder from the given node
func NewSectorBuilder(nd *Node) *SectorBuilder {
	g := drgraph.New(sectorSize / nodeSize)
	return &SectorBuilder{
		G:         g,
		Prover:    drg.NewProver("cats", g, nodeSize),
		CurSector: new(Sector),
		nd:        nd,
	}
}

// AddPiece writes the given piece into a sector
func (smc *SectorBuilder) AddPiece(ctx context.Context, pi *PieceInfo) error {
	if smc.CurSector.Free < pi.Size {
		curSector := smc.CurSector
		smc.CurSector = new(Sector)
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

// SealAndPostSector seals the given sector and posts its commitment on-chain
func (smc *SectorBuilder) SealAndPostSector(ctx context.Context, s *Sector) {
	ss, err := s.Seal(smc.MinerAddr, filecoinParameters)
	if err != nil {
		// Hard to say what to do in this case.
		// Depending on the error, it could be "try again"
		// or 'verify data integrity and try again'
		panic(err)
	}

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
		-1, // NB: we might already know the sector ID from having created it already
		ss.merkleRoot,
		deals,
	)
	if err != nil {
		return err
	}

	minerOwner := types.Address{} // TODO: get the miner owner to send this from
	msg := types.NewMessage(minerOwner, smc.MinerAddr, nil, "commitSector", params)

	if err := smc.nd.AddNewMessage(ctx, msg); err != nil {
		return errors.Wrap(err, "pushing out commitSector message failed")
	}

	// TODO: maybe wait for the message to clear on chain?
	return nil
}

// WritePiece writes data from the given reader to the sectors underlying storage
func (s *Sector) WritePiece(pi *PieceInfo, r io.Reader) error {
	/*
		n, err := io.Copy(s.file, r)
		if err != nil {
			// TODO: make sure we roll back the state of the file to what it was before we started this write
			// Note, this should be a fault. We likely should already have all the
			// data locally before calling this method. If that ends up being the
			// case, this error signifies disk errors of some kind
			return err
		}
	*/
	// TODO: this is temporary. Use the above code later
	// We should be writing this out to a file in the 'staging' area. Once the
	// sector is ready to be sealed, we should mmap it and pass it to the
	// sealing code. The sealing code prefers fairly random access to the data,
	// so using mmap will be the fastest option. We could also provide an
	// interface to just do seeks and reads on the underlying file, but it
	// won't be as optimized
	alldata, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	s.data = append(s.data, alldata...)
	//

	dataSize := uint64(len(alldata))

	if /* uint64(n)  */ dataSize != pi.Size {
		// TODO: make sure we roll back the state of the file to what it was before we started this write
		return fmt.Errorf("reported piece size does not match what we read")
	}

	s.Free -= dataSize
	s.Pieces = append(s.Pieces, pi)

	return nil
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
}

// Seal runs the PoRep setup process using the given parameters
// address may turn into a secret key
func (s *Sector) Seal(minerID types.Address, publicParams *PublicParameters) (*SealedSector, error) {
	if len(s.data) < sectorSize {
		s.data = append(s.data, make([]byte, sectorSize-len(s.data))...)
	}
	prover := drg.NewProver(string(minerID[:]), publicParams.graph, int(publicParams.blockSize))
	_, root := prover.Setup(s.data)
	return &SealedSector{
		replicaData: prover.Replica,
		merkleRoot:  root,
		baseSector:  s,
	}, nil
}
