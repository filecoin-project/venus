package testing

import (
	"io/ioutil"
	"testing"

	bserv "github.com/ipfs/go-blockservice"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/proofs/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/repo"

	"github.com/stretchr/testify/require"
)

// Builder is used to create a SectorBuilder test harness
type Builder struct {
	t          *testing.T
	stagingDir string
	sealedDir  string
}

// NewBuilder dispenses a harness builder
func NewBuilder(t *testing.T) *Builder {
	return &Builder{
		sealedDir:  "",
		stagingDir: "",
		t:          t,
	}
}

// StagingDir sets the builder's staging directory
func (b *Builder) StagingDir(stagingDir string) *Builder {
	b.stagingDir = stagingDir

	return b
}

// SealedDir sets the builder's staging directory
func (b *Builder) SealedDir(sealedDir string) *Builder {
	b.sealedDir = sealedDir

	return b
}

// Build consumes builder and produces a new testing harness
func (b *Builder) Build() Harness {
	if b.stagingDir == "" {
		stagingDir, err := ioutil.TempDir("", "staging")
		if err != nil {
			panic(err)
		}

		b.stagingDir = stagingDir
	}

	if b.sealedDir == "" {
		sealedDir, err := ioutil.TempDir("", "sealed")
		if err != nil {
			panic(err)
		}

		b.sealedDir = sealedDir
	}

	memRepo := repo.NewInMemoryRepoWithSectorDirectories(b.stagingDir, b.sealedDir)
	blockStore := bstore.NewBlockstore(memRepo.Datastore())
	blockService := bserv.New(blockStore, offline.Exchange(blockStore))
	minerAddr := address.MakeTestAddress("wombat")

	// TODO: Replace this with proofs.Live plus a sector size (in this case,
	// "small" or 127 (bytes).
	sectorStoreType := proofs.Test

	sb, err := sectorbuilder.NewRustSectorBuilder(sectorbuilder.RustSectorBuilderConfig{
		BlockService:     blockService,
		LastUsedSectorID: 0,
		MetadataDir:      memRepo.StagingDir(),
		MinerAddr:        minerAddr,
		SealedSectorDir:  memRepo.SealedDir(),
		SectorStoreType:  sectorStoreType,
		StagedSectorDir:  memRepo.StagingDir(),
	})
	require.NoError(b.t, err)

	n, err := sb.GetMaxUserBytesPerStagedSector()
	require.NoError(b.t, err)

	return Harness{
		t:                 b.t,
		repo:              memRepo,
		blockService:      blockService,
		SectorBuilder:     sb,
		MinerAddr:         minerAddr,
		MaxBytesPerSector: n,
		SectorConfig:      sectorStoreType,
	}
}
