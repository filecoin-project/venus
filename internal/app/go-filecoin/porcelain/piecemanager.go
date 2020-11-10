package porcelain

import (
	"context"
	"io"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/internal/pkg/piecemanager"
)

type pmPlumbing interface {
	PieceManager() piecemanager.PieceManager
}

// SealPieceIntoNewSector writes the provided piece-bytes into a new sector.
func SealPieceIntoNewSector(ctx context.Context, p pmPlumbing, dealID abi.DealID, dealStart, dealEnd abi.ChainEpoch, pieceSize abi.UnpaddedPieceSize, pieceReader io.Reader) (*storagemarket.PackingResult, error) {
	if p.PieceManager() == nil {
		return nil, errors.New("must be mining to add piece")
	}

	return p.PieceManager().SealPieceIntoNewSector(ctx, dealID, dealStart, dealEnd, pieceSize, pieceReader)
}
