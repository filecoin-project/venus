package testing

import (
	"io/ioutil"
	"testing"

	bserv "github.com/ipfs/go-blockservice"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/proofs/libsectorbuilder"
	"github.com/filecoin-project/go-filecoin/proofs/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"

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

	memRepo := repo.NewInMemoryRepo()
	blockStore := bstore.NewBlockstore(memRepo.Datastore())
	blockService := bserv.New(blockStore, offline.Exchange(blockStore))
	minerAddr, err := address.NewActorAddress([]byte("wombat"))
	if err != nil {
		panic(err)
	}

	class := types.NewSectorClass(types.OneKiBSectorSize)

	sb, err := sectorbuilder.NewRustSectorBuilder(sectorbuilder.RustSectorBuilderConfig{
		BlockService:     blockService,
		LastUsedSectorID: 0,
		MetadataDir:      b.stagingDir,
		MinerAddr:        minerAddr,
		SealedSectorDir:  b.sealedDir,
		SectorClass:      class,
		StagedSectorDir:  b.stagingDir,
	})
	require.NoError(b.t, err)

	max := types.NewBytesAmount(libsectorbuilder.GetMaxUserBytesPerStagedSector(class.SectorSize().Uint64()))
	require.NoError(b.t, err)

	return Harness{
		t:                 b.t,
		repo:              memRepo,
		blockService:      blockService,
		SectorBuilder:     sb,
		MinerAddr:         minerAddr,
		MaxBytesPerSector: max,
	}
}
