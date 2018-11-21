package sectorbuilder

import (
	"context"
	"crypto/rand"
	"io"
	"strings"
	"testing"

	bserv "gx/ipfs/QmTfTKeBhTLjSjxXQsjkF2b1DfZmYEMnknGE2y2gX57C6v/go-blockservice"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	"gx/ipfs/QmZxjqR9Qgompju73kakSoUj3rbVndAzky3oCDiBNCxPs1/go-ipfs-exchange-offline"
	bstore "gx/ipfs/QmcmpX42gtDv1fz24kau4wjS9hfwWj5VexWBKgGnWzsyag/go-ipfs-blockstore"
	dag "gx/ipfs/QmeLG6jF1xvEmHca5Vy4q4EdQWp8Xq9S6EPyZrN9wvSRLC/go-merkledag"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/repo"

	"github.com/stretchr/testify/require"
)

type sectorBuilderType int

const (
	rust = sectorBuilderType(iota)
	golang
)

type sectorBuilderTestHarness struct {
	blockService      bserv.BlockService
	ctx               context.Context
	minerAddr         address.Address
	repo              repo.Repo
	sectorBuilder     SectorBuilder
	t                 *testing.T
	maxBytesPerSector uint64
}

func newSectorBuilderTestHarness(ctx context.Context, t *testing.T, cfg sectorBuilderType) sectorBuilderTestHarness { // nolint: deadcode
	memRepo := repo.NewInMemoryRepo()
	blockStore := bstore.NewBlockstore(memRepo.Datastore())
	blockService := bserv.New(blockStore, offline.Exchange(blockStore))
	sectorStore := proofs.NewProofTestSectorStore(memRepo.StagingDir(), memRepo.SealedDir())
	minerAddr := address.MakeTestAddress("wombat")

	var sectorBuilder SectorBuilder
	if cfg == golang {
		sb, err := Init(ctx, memRepo.Datastore(), blockService, minerAddr, sectorStore, 0)
		require.NoError(t, err)

		sectorBuilder = sb
	} else if cfg == rust {
		sb, err := NewRustSectorBuilder(RustSectorBuilderConfig{
			BlockService:     blockService,
			LastUsedSectorID: 0,
			MetadataDir:      memRepo.StagingDir(),
			MinerAddr:        minerAddr,
			SealedSectorDir:  memRepo.SealedDir(),
			SectorStoreType:  proofs.ProofTest,
			StagedSectorDir:  memRepo.StagingDir(),
		})
		require.NoError(t, err)

		sectorBuilder = sb
	} else {
		t.Fatalf("unhandled sector builder type: %v", cfg)
	}

	numBytes, err := sectorBuilder.GetMaxUserBytesPerStagedSector()
	require.NoError(t, err)

	return sectorBuilderTestHarness{
		ctx:               ctx,
		t:                 t,
		repo:              memRepo,
		blockService:      blockService,
		sectorBuilder:     sectorBuilder,
		minerAddr:         minerAddr,
		maxBytesPerSector: numBytes,
	}
}

func (h sectorBuilderTestHarness) close() {
	err1 := h.sectorBuilder.Close()
	err2 := h.repo.Close()

	var msg []string
	if err1 != nil {
		msg = append(msg, err1.Error())
	}

	if err2 != nil {
		msg = append(msg, err2.Error())
	}

	if len(msg) != 0 {
		h.t.Fatalf("error(s) closing harness: %s", strings.Join(msg, " and "))
	}
}

func (h sectorBuilderTestHarness) requireAddPiece(pieceData []byte) *cid.Cid {
	pieceInfo, err := h.createPieceInfo(pieceData)
	require.NoError(h.t, err)

	_, err = h.sectorBuilder.AddPiece(h.ctx, pieceInfo)
	require.NoError(h.t, err)

	return pieceInfo.Ref
}

func (h sectorBuilderTestHarness) createPieceInfo(pieceData []byte) (*PieceInfo, error) {
	data := dag.NewRawNode(pieceData)

	if err := h.blockService.AddBlock(data); err != nil {
		return nil, err
	}

	return &PieceInfo{
		Ref:  data.Cid(),
		Size: uint64(len(pieceData)),
	}, nil
}

func requireRandomBytes(t *testing.T, n uint64) []byte { // nolint: deadcode
	slice := make([]byte, n)

	_, err := io.ReadFull(rand.Reader, slice)
	require.NoError(t, err)

	return slice
}
