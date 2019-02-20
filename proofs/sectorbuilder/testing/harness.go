package testing

import (
	"context"
	"crypto/rand"
	"io"
	"strings"
	"testing"

	dag "gx/ipfs/QmNRAuGmvnVw8urHkUZQirhu42VTiZjVWASa2aTznEMmpP/go-merkledag"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	bserv "gx/ipfs/QmZsGVGCqMCNzHLNMB6q4F6yyvomqf1VxwhJwSfgo1NGaF/go-blockservice"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/proofs"
	sb "github.com/filecoin-project/go-filecoin/proofs/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/repo"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

// Harness is a struct used to make SectorBuilder testing easier
type Harness struct {
	blockService      bserv.BlockService
	MaxBytesPerSector uint64
	MinerAddr         address.Address
	repo              repo.Repo
	SectorBuilder     sb.SectorBuilder
	SectorConfig      proofs.SectorStoreType
	t                 *testing.T
}

// Close cleans up the resources used by the harness
func (h Harness) Close() {
	err1 := h.SectorBuilder.Close()
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

// AddPiece adds the provided bytes to the underlying SectorBuilder and returns
// the sector id, piece cid, and any error
func (h Harness) AddPiece(ctx context.Context, pieceData []byte) (uint64, cid.Cid, error) {
	pieceInfo, err := h.CreatePieceInfo(pieceData)
	if err != nil {
		return 0, cid.Undef, err
	}

	sectorID, err := h.SectorBuilder.AddPiece(ctx, pieceInfo)
	if err != nil {
		return 0, cid.Undef, err
	}

	return sectorID, pieceInfo.Ref, nil
}

// CreatePieceInfo creates a PieceInfo for the provided bytes
func (h Harness) CreatePieceInfo(pieceData []byte) (*sb.PieceInfo, error) {
	data := dag.NewRawNode(pieceData)

	if err := h.blockService.AddBlock(data); err != nil {
		return nil, err
	}

	return &sb.PieceInfo{
		Ref:  data.Cid(),
		Size: uint64(len(pieceData)),
	}, nil
}

// RequireRandomBytes produces n-number of bytes
func RequireRandomBytes(t *testing.T, n uint64) []byte { // nolint: deadcode
	slice := make([]byte, n)

	_, err := io.ReadFull(rand.Reader, slice)
	require.NoError(t, err)

	return slice
}
