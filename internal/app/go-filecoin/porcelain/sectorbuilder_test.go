package porcelain_test

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-sectorbuilder"
)

func TestSealNow(t *testing.T) {
	t.Run("triggers sealing", func(t *testing.T) {
		p := newTestSectorBuilderPlumbing(1)

		err := SealNow(context.Background(), p)
		require.NoError(t, err)

		// seals sectors
		assert.Equal(t, 1, p.sectorbuilder.sealAllSectorsCount)
	})
}

func TestAddPiece(t *testing.T) {
	t.Run("adds piece", func(t *testing.T) {
		p := newTestSectorBuilderPlumbing(0)

		assert.Equal(t, 0, p.sectorbuilder.addPieceCount)

		_, err := AddPiece(context.Background(), p, bytes.NewReader(testPieceData))
		require.NoError(t, err)

		// adds a piece
		assert.Equal(t, 1, p.sectorbuilder.addPieceCount)

		// doesn't seal sectors
		assert.Equal(t, 0, p.sectorbuilder.sealAllSectorsCount)
	})
}

func newTestSectorBuilderPlumbing(stagedSectors int) *testSectorBuilderPlumbing {
	sb := &testSectorBuilder{numStagedSectors: stagedSectors}
	return &testSectorBuilderPlumbing{
		sectorbuilder: sb,
	}
}

type testSectorBuilderPlumbing struct {
	sectorbuilder *testSectorBuilder
}

var testPieceData = []byte{1, 2, 3}

func (tsbp *testSectorBuilderPlumbing) SectorBuilder() sectorbuilder.SectorBuilder {
	return tsbp.sectorbuilder
}

func (tsbp *testSectorBuilderPlumbing) DAGImportData(ctx context.Context, pieceReader io.Reader) (ipld.Node, error) {
	return cbor.WrapObject(testPieceData, types.DefaultHashFunction, -1)
}

func (tsbp *testSectorBuilderPlumbing) DAGCat(ctx context.Context, cid cid.Cid) (io.Reader, error) {
	return bytes.NewReader(testPieceData), nil
}

func (tsbp *testSectorBuilderPlumbing) DAGGetFileSize(ctx context.Context, c cid.Cid) (uint64, error) {
	return uint64(len(testPieceData)), nil
}

type testSectorBuilder struct {
	addPieceCount       int
	sealAllSectorsCount int
	numStagedSectors    int
}

func (tsb *testSectorBuilder) AddPiece(ctx context.Context, pieceRef cid.Cid, pieceSize uint64, pieceReader io.Reader) (sectorID uint64, err error) {
	tsb.addPieceCount++
	return 0, nil
}

func (tsb *testSectorBuilder) ReadPieceFromSealedSector(pieceCid cid.Cid) (io.Reader, error) {
	return nil, nil
}

func (tsb *testSectorBuilder) SealAllStagedSectors(ctx context.Context) error {
	tsb.sealAllSectorsCount++
	return nil
}

func (tsb *testSectorBuilder) GetAllStagedSectors() ([]go_sectorbuilder.StagedSectorMetadata, error) {
	return make([]go_sectorbuilder.StagedSectorMetadata, tsb.numStagedSectors), nil
}

func (tsb *testSectorBuilder) SectorSealResults() <-chan sectorbuilder.SectorSealResult {
	return nil
}

func (tsb *testSectorBuilder) GeneratePoSt(sectorbuilder.GeneratePoStRequest) (sectorbuilder.GeneratePoStResponse, error) {
	return sectorbuilder.GeneratePoStResponse{}, nil
}

func (tsb *testSectorBuilder) Close() error {
	return nil
}
