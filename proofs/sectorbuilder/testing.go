package sectorbuilder

import (
	"context"
	"crypto/rand"
	"io"
	"strings"
	"testing"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	bstore "gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"
	bserv "gx/ipfs/QmVDTbzzTwnuBwNbJdhW3u7LoBQp46bezm9yp4z1RoEepM/go-blockservice"
	"gx/ipfs/QmYZwey1thDTynSrvd6qQkX24UpTka6TFhQ2v569UpoqxD/go-ipfs-exchange-offline"
	dag "gx/ipfs/QmdURv6Sbob8TVW2tFFve9vcEWrSUgwPqeqnXyvYhLrkyd/go-merkledag"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/repo"

	"github.com/stretchr/testify/require"
)

type sectorBuilderTestHarness struct {
	blockService      bserv.BlockService
	ctx               context.Context
	maxBytesPerSector uint64
	minerAddr         address.Address
	repo              repo.Repo
	sectorBuilder     SectorBuilder
	sectorStoreType   proofs.SectorStoreType
	t                 *testing.T
}

func newSectorBuilderTestHarness(ctx context.Context, t *testing.T) sectorBuilderTestHarness { // nolint: deadcode
	memRepo := repo.NewInMemoryRepo()
	blockStore := bstore.NewBlockstore(memRepo.Datastore())
	blockService := bserv.New(blockStore, offline.Exchange(blockStore))
	minerAddr := address.MakeTestAddress("wombat")

	// TODO: Replace this with proofs.Live plus a sector size (in this case,
	// "small" or 127 (bytes).
	sectorStoreType := proofs.ProofTest

	sb, err := NewRustSectorBuilder(RustSectorBuilderConfig{
		BlockService:     blockService,
		LastUsedSectorID: 0,
		MetadataDir:      memRepo.StagingDir(),
		MinerAddr:        minerAddr,
		SealedSectorDir:  memRepo.SealedDir(),
		SectorStoreType:  sectorStoreType,
		StagedSectorDir:  memRepo.StagingDir(),
	})
	require.NoError(t, err)

	n, err := sb.GetMaxUserBytesPerStagedSector()
	require.NoError(t, err)

	return sectorBuilderTestHarness{
		ctx:               ctx,
		t:                 t,
		repo:              memRepo,
		blockService:      blockService,
		sectorBuilder:     sb,
		minerAddr:         minerAddr,
		maxBytesPerSector: n,
		sectorStoreType:   sectorStoreType,
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

func (h sectorBuilderTestHarness) addPiece(pieceData []byte) (cid.Cid, error) {
	pieceInfo, err := h.createPieceInfo(pieceData)
	if err != nil {
		return cid.Undef, err
	}

	_, err = h.sectorBuilder.AddPiece(h.ctx, pieceInfo)
	if err != nil {
		return cid.Undef, err
	}

	return pieceInfo.Ref, nil
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
