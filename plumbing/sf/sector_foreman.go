package sf

import (
	"os"

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
