package testing

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"strings"
	"testing"

	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	dag "github.com/ipfs/go-merkledag"

	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	sb "github.com/filecoin-project/go-filecoin/internal/pkg/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/stretchr/testify/require"
)

// Harness is a struct used to make SectorBuilder testing easier
type Harness struct {
	blockService      bserv.BlockService
	MaxBytesPerSector *types.BytesAmount
	MinerAddr         address.Address
	repo              repo.Repo
	SectorBuilder     sb.SectorBuilder
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
	ref, size, reader, err := h.CreateAddPieceArgs(pieceData)
	if err != nil {
		return 0, cid.Undef, err
	}

	sectorID, err := h.SectorBuilder.AddPiece(ctx, ref, size, reader)
	if err != nil {
		return 0, cid.Undef, err
	}

	return sectorID, ref, nil
}

// CreateAddPieceArgs creates a PieceInfo for the provided bytes
func (h Harness) CreateAddPieceArgs(pieceData []byte) (cid.Cid, uint64, io.Reader, error) {
	data := dag.NewRawNode(pieceData)

	if err := h.blockService.AddBlock(data); err != nil {
		return cid.Cid{}, 0, nil, err
	}

	return data.Cid(), uint64(len(pieceData)), bytes.NewReader(pieceData), nil
}

// RequireRandomBytes produces n-number of bytes
func RequireRandomBytes(t *testing.T, n uint64) []byte { // nolint: deadcode
	slice := make([]byte, n)

	_, err := io.ReadFull(rand.Reader, slice)
	require.NoError(t, err)

	return slice
}
