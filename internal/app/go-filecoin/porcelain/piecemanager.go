package porcelain

import (
	"context"
	"io"

	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/piecemanager"
)

type pmPlumbing interface {
	PieceManager() piecemanager.PieceManager
}

// SealPieceIntoNewSector writes the provided piece-bytes into a new sector.
func SealPieceIntoNewSector(ctx context.Context, p pmPlumbing, dealID uint64, pieceSize uint64, pieceReader io.Reader) error {
	if p.PieceManager() == nil {
		return errors.New("must be mining to add piece")
	}

	return p.PieceManager().SealPieceIntoNewSector(ctx, dealID, pieceSize, pieceReader)
}
