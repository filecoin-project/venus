package porcelain

import (
	"context"
	"io"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/piecemanager"
)

type pmPlumbing interface {
	PieceManager() piecemanager.PieceManager
}

// SealPieceIntoNewSector writes the provided piece-bytes into a new sector.
func SealPieceIntoNewSector(ctx context.Context, p pmPlumbing, dealID abi.DealID, dealStart, dealEnd abi.ChainEpoch, pieceSize uint64, pieceReader io.Reader) error {
	if p.PieceManager() == nil {
		return errors.New("must be mining to add piece")
	}

	return p.PieceManager().SealPieceIntoNewSector(ctx, dealID, dealStart, dealEnd, pieceSize, pieceReader)
}
