package sectorbuilder

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"strings"
	"testing"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	bstore "gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"
	bserv "gx/ipfs/QmVDTbzzTwnuBwNbJdhW3u7LoBQp46bezm9yp4z1RoEepM/go-blockservice"
	offline "gx/ipfs/QmYZwey1thDTynSrvd6qQkX24UpTka6TFhQ2v569UpoqxD/go-ipfs-exchange-offline"
	dag "gx/ipfs/QmdURv6Sbob8TVW2tFFve9vcEWrSUgwPqeqnXyvYhLrkyd/go-merkledag"

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

	var maxBytes uint64
	var sectorBuilder SectorBuilder
	if cfg == golang {
		sb, err := Init(ctx, memRepo.Datastore(), blockService, minerAddr, sectorStore, 0)
		require.NoError(t, err)

		sectorBuilder = sb

		response, err := sectorStore.GetMaxUnsealedBytesPerSector()
		require.NoError(t, err)

		maxBytes = response.NumBytes
	} else if cfg == rust {
		sb, err := NewRustSectorBuilder(RustSectorBuilderConfig{
			blockService:        blockService,
			lastUsedSectorID:    0,
			metadataDir:         memRepo.StagingDir(),
			proverID:            [31]byte{},
			sealedSectorDir:     memRepo.SealedDir(),
			sectorStoreType:     proofs.ProofTest,
			stagedSectorDir:     memRepo.StagingDir(),
			maxNumStagedSectors: 1,
		})
		require.NoError(t, err)

		sectorBuilder = sb

		n, err := sb.GetMaxUserBytesPerStagedSector()
		require.NoError(t, err)

		maxBytes = n
	} else {
		t.Fatalf("unhandled sector builder type: %v", cfg)
	}

	return sectorBuilderTestHarness{
		ctx:               ctx,
		t:                 t,
		repo:              memRepo,
		blockService:      blockService,
		sectorBuilder:     sectorBuilder,
		minerAddr:         minerAddr,
		maxBytesPerSector: maxBytes,
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

func (h sectorBuilderTestHarness) addPiece(pieceData []byte) (*cid.Cid, error) {
	pieceInfo, err := h.createPieceInfo(pieceData)
	if err != nil {
		return nil, err
	}

	_, err = h.sectorBuilder.AddPiece(h.ctx, pieceInfo)
	if err != nil {
		return nil, err
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

func metadataMustMatch(require *require.Assertions, sb *defaultSectorBuilder, sector *UnsealedSector, sealed *SealedSector, pieces int) { // nolint: deadcode
	if sealed != nil {
		sealedMeta := sealed.dumpCurrentState()
		sealedMetaPersisted, err := sb.metadataStore.getSealedSectorMetadata(sealed.CommR)
		require.NoError(err)
		require.Equal(sealedMeta, sealedMetaPersisted)
	} else {
		meta := sector.dumpCurrentState()
		require.Len(meta.Pieces, pieces)

		// persisted and calculated metadata match.
		metaPersisted, err := sb.metadataStore.getSectorMetadata(sector.unsealedSectorAccess)
		require.NoError(err)
		require.Equal(metaPersisted, meta)
	}

	builderMeta := sb.dumpCurrentState()
	builderMetaPersisted, err := sb.metadataStore.getSectorBuilderMetadata(sb.minerAddr)
	require.NoError(err)
	require.Equal(builderMeta, builderMetaPersisted)
}

func pieceInfoMustEqual(t *testing.T, p1 *PieceInfo, p2 *PieceInfo) {
	if p1.Size != p2.Size {
		t.Fatalf("p1.size(%d) != p2.size(%d)\n", p1.Size, p2.Size)
	}

	if !p1.Ref.Equals(p2.Ref) {
		t.Fatalf("p1.Ref(%s) != p2.Ref(%s)\n", p1.Ref.String(), p2.Ref.String())
	}
}

func sectorBuildersMustEqual(t *testing.T, sb1 *defaultSectorBuilder, sb2 *defaultSectorBuilder) { // nolint: deadcode
	require := require.New(t)

	require.Equal(sb1.minerAddr, sb2.minerAddr)
	require.Equal(sb1.sectorSize, sb2.sectorSize)

	sectorsMustEqual(t, sb1.curUnsealedSector, sb2.curUnsealedSector)

	require.Equal(len(sb1.sealedSectors), len(sb2.sealedSectors))
	for i := 0; i < len(sb1.sealedSectors); i++ {
		sealedSectorsMustEqual(t, sb1.sealedSectors[i], sb2.sealedSectors[i])
	}
}

func sealedSectorsMustEqual(t *testing.T, ss1 *SealedSector, ss2 *SealedSector) {
	require := require.New(t)

	if ss1 == nil && ss2 == nil {
		return
	}

	require.Equal(ss1.sealedSectorAccess, ss2.sealedSectorAccess)
	require.Equal(ss1.unsealedSectorAccess, ss2.unsealedSectorAccess)
	require.Equal(ss1.numBytes, ss2.numBytes)
	require.True(bytes.Equal(ss1.CommR[:], ss2.CommR[:]))

	require.Equal(len(ss1.pieces), len(ss2.pieces))
	for i := 0; i < len(ss1.pieces); i++ {
		pieceInfoMustEqual(t, ss1.pieces[i], ss2.pieces[i])
	}
}

func sectorsMustEqual(t *testing.T, s1 *UnsealedSector, s2 *UnsealedSector) {
	require := require.New(t)

	require.Equal(s1.unsealedSectorAccess, s2.unsealedSectorAccess)
	require.Equal(s1.maxBytes, s2.maxBytes)
	require.Equal(s1.numBytesUsed, s2.numBytesUsed)

	require.Equal(len(s1.pieces), len(s2.pieces))
	for i := 0; i < len(s1.pieces); i++ {
		pieceInfoMustEqual(t, s1.pieces[i], s2.pieces[i])
	}
}

func requireRandomBytes(t *testing.T, n uint64) []byte { // nolint: deadcode
	slice := make([]byte, n)

	_, err := io.ReadFull(rand.Reader, slice)
	require.NoError(t, err)

	return slice
}
