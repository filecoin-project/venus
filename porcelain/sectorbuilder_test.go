package porcelain_test

import (
	"context"
	"io"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/proofs/sectorbuilder"
	"github.com/filecoin-project/go-sectorbuilder"
)

func TestSealNow(t *testing.T) {
	t.Run("adds piece and triggers sealing when staged sectors is empty", func(t *testing.T) {
		p := newTestSectorBuilderPlumbing(0)

		err := SealNow(context.Background(), p)
		require.NoError(t, err)

		// adds a piece
		assert.Equal(t, 1, p.sectorbuilder.addPieceCount)

		// seals sectors
		assert.Equal(t, 1, p.sectorbuilder.sealAllSectorsCount)
	})

	t.Run("does not add a piece when staged sectors exist", func(t *testing.T) {
		p := newTestSectorBuilderPlumbing(4)

		err := SealNow(context.Background(), p)
		require.NoError(t, err)

		// does not add a piece
		assert.Equal(t, 0, p.sectorbuilder.addPieceCount)

		// seals sectors
		assert.Equal(t, 1, p.sectorbuilder.sealAllSectorsCount)
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

func (tsbp *testSectorBuilderPlumbing) SectorBuilder() sectorbuilder.SectorBuilder {
	return tsbp.sectorbuilder
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
