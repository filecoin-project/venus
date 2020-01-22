package piecemanager

import (
	"context"
	"io"
)

// PieceManager is responsible for sealing pieces into sectors and progressing
// the sector through its lifecycle, including coordinating with the node for
// purposes of creating pre-commit and commit messages.
type PieceManager interface {
	// SealPieceIntoNewSector writes the provided piece into a sector and fills
	// the remaining space in the sector with self deal-data. The now-filled
	// sector is encoded and, when the required bits of chain randomness are
	// available, committed to the network. This method is fire-and-forget; any
	// errors encountered during the pre-commit or commit flows (including
	// message creation) are recorded in StorageMining metadata but not exposed
	// through this API.
	SealPieceIntoNewSector(ctx context.Context, dealID uint64, pieceSize uint64, pieceReader io.Reader) error

	// PledgeSector behaves similarly to SealPieceIntoNewSector, but differs in
	// that it does not require a deal having been made on-chain beforehand. It
	// provisions a new sector, fills it with self-deal junk, and seals. Like
	// SealPieceIntoNewSector, this method is fire-and-forget.
	PledgeSector() error

	// UnsealSector produces a reader to the unsealed bytes associated with the
	// provided sector id, or an error if no such sealed sector exists. The
	// bytes produced by the Reader will not include any bit-padding.
	UnsealSector(ctx context.Context, sectorID uint64) (io.ReadCloser, error)

	// LocatePieceForDealWithinSector produces information about the location of
	// a deal's piece within a sealed sector, or an error if that piece does not
	// exist within any sealed sectors.
	LocatePieceForDealWithinSector(ctx context.Context, dealID uint64) (sectorID uint64, offset uint64, length uint64, err error)
}
