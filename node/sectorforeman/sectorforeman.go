package sectorforeman

import (
	"context"
	"io"
	"os"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	bserv "gx/ipfs/QmZsGVGCqMCNzHLNMB6q4F6yyvomqf1VxwhJwSfgo1NGaF/go-blockservice"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/proofs/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/repo"
)

// SectorForeman manages sector builders, get it?
type SectorForeman struct {
	bs            bserv.BlockService
	repo          repo.Repo
	sectorBuilder sectorbuilder.SectorBuilder
}

// NewSectorForeman returns a new SectorForeman
func NewSectorForeman(repo repo.Repo, bs bserv.BlockService) *SectorForeman {
	return &SectorForeman{
		repo: repo,
		bs:   bs,
	}
}

// Start accepts a miner address and last used sector id then starts a
// SectorBuilder with it. If mining has already started, it returns an error.
func (sf *SectorForeman) Start(minerAddr address.Address, lastUsedSectorID uint64) error {
	if sf.sectorBuilder != nil {
		return errors.New("sectorbuilder already started")
	}

	sectorStoreType := proofs.Live
	if os.Getenv("FIL_USE_SMALL_SECTORS") == "true" {
		sectorStoreType = proofs.Test
	}

	// TODO: Where should we store the RustSectorBuilder metadata? Currently, we
	// configure the RustSectorBuilder to store its metadata in the staging
	// directory.

	sb, err := sectorbuilder.NewRustSectorBuilder(sectorbuilder.RustSectorBuilderConfig{
		BlockService:     sf.bs,
		LastUsedSectorID: lastUsedSectorID,
		MetadataDir:      sf.repo.StagingDir(),
		MinerAddr:        minerAddr,
		SealedSectorDir:  sf.repo.SealedDir(),
		SectorStoreType:  sectorStoreType,
		StagedSectorDir:  sf.repo.StagingDir(),
	})
	if err == nil {
		sf.sectorBuilder = sb
	}

	return err
}

// Stop stops the sectorBuilder and sets it to nil
func (sf *SectorForeman) Stop() error {
	if sf.sectorBuilder != nil {
		if err := sf.sectorBuilder.Close(); err != nil {
			return errors.Wrap(err, "error closing sector builder")
		}
		sf.sectorBuilder = nil
	}
	return nil
}

// IsRunning returns true if the sectorbuilder is present, otherwise false
func (sf *SectorForeman) IsRunning() bool {
	return sf.sectorBuilder != nil
}

// SealAllStagedSectors seals all stages sectors on the sector builder
func (sf *SectorForeman) SealAllStagedSectors(ctx context.Context) error {
	return sf.sectorBuilder.SealAllStagedSectors(ctx)
}

// SectorSealResults gets seal results from the sector builder
func (sf *SectorForeman) SectorSealResults() <-chan sectorbuilder.SectorSealResult {
	return sf.sectorBuilder.SectorSealResults()
}

// ReadPieceFromSealedSector reads a piece from a sealed sector
func (sf *SectorForeman) ReadPieceFromSealedSector(pieceCid cid.Cid) (io.Reader, error) {
	return sf.sectorBuilder.ReadPieceFromSealedSector(pieceCid)
}

// AddPiece adds a piece to the sectorbuilder
func (sf *SectorForeman) AddPiece(ctx context.Context, pi *sectorbuilder.PieceInfo) (sectorID uint64, err error) {
	return sf.sectorBuilder.AddPiece(ctx, pi)
}

// GeneratePoST generates proof of spacetime for the sectorbuilder
func (sf *SectorForeman) GeneratePoST(req sectorbuilder.GeneratePoSTRequest) (sectorbuilder.GeneratePoSTResponse, error) {
	return sf.sectorBuilder.GeneratePoST(req)
}
