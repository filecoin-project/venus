package piecemanager

import (
	"context"
	"io"

	"github.com/filecoin-project/go-sectorbuilder"
	storagenode "github.com/filecoin-project/go-storage-miner/apis/node"
	storage "github.com/filecoin-project/go-storage-miner/sealing"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/pkg/errors"
)

// StorageMinerAPI provides an interface to the bits of a storage miner required
// by the StorageMinerBackEnd, which satisfies PieceManager interface.
type StorageMinerAPI interface {
	// AllocateSectorID allocates a new sector ID.
	AllocateSectorID() (abi.SectorNumber, error)

	// PledgeSector allocates a new sector, fills it with self-deal junk, and
	// seals that sector.
	PledgeSector(ctx context.Context) error

	// SealPiece writes the provided piece to a newly-created sector which it
	// immediately seals.
	SealPiece(ctx context.Context, size abi.UnpaddedPieceSize, r io.Reader, sectorNum abi.SectorNumber, pdi storagenode.DealInfo) error

	// GetSectorInfo produces information about a sector managed by this storage
	// miner, or an error if the miner does not manage a sector with the
	// provided identity.
	GetSectorInfo(abi.SectorNumber) (storage.SectorInfo, error)

	// ListSectors lists all the sectors managed by this storage miner (sealed
	// or otherwise).
	ListSectors() ([]storage.SectorInfo, error)
}

// SectorBuilderAPI defines the subset of the sector builder API required by the
// StorageMinerBackEnd.
type SectorBuilderAPI interface {
	SectorSize() abi.SectorSize
	ReadPieceFromSealedSector(ctx context.Context, sectorNumber abi.SectorNumber, pieceIndex sectorbuilder.UnpaddedByteIndex, pieceSize abi.UnpaddedPieceSize, ticketBytes []byte, commD []byte) (io.ReadCloser, error)
}

// StorageMinerBackEnd is...
type StorageMinerBackEnd struct {
	miner   StorageMinerAPI
	builder SectorBuilderAPI
}

// NewStorageMinerBackEnd produces a new StorageMinerBackEnd
func NewStorageMinerBackEnd(m StorageMinerAPI, b SectorBuilderAPI) *StorageMinerBackEnd {
	return &StorageMinerBackEnd{
		miner:   m,
		builder: b,
	}
}

// SealPieceIntoNewSector provisions a new sector and writes the provided piece
// to that sector. Any remaining space in the sector is filled with self-deals,
// and the sector is committed to the network automatically and asynchronously.
func (s *StorageMinerBackEnd) SealPieceIntoNewSector(ctx context.Context, dealID uint64, dealStart, dealEnd abi.ChainEpoch, pieceSize uint64, pieceReader io.Reader) error {
	sectorID, err := s.miner.AllocateSectorID()
	if err != nil {
		return errors.Wrap(err, "failed to acquire sector id from storage miner")
	}

	err = s.miner.SealPiece(ctx, abi.UnpaddedPieceSize(pieceSize), pieceReader, sectorID, storagenode.DealInfo{
		DealID: abi.DealID(dealID),
		DealSchedule: storagenode.DealSchedule{
			StartEpoch: dealStart,
			EndEpoch:   dealEnd,
		},
	})
	if err != nil {
		return errors.Wrap(err, "storage miner `SealPiece` produced an error")
	}

	return nil
}

// PledgeSector delegates to the go-storage-miner PledgeSector method.
func (s *StorageMinerBackEnd) PledgeSector(ctx context.Context) error {
	return s.miner.PledgeSector(ctx)
}

// UnsealSector uses the blockchain to acquire the ticket and commD associated
// with the pre-commit message for the provided sector id and miner for purposes
// of unsealing the sector. This method returns a reader to the plaintext user-
// bytes (bit-padding removed), or an error if - from the chain's perspective -
// the sector was not pre-committed by the miner.
func (s *StorageMinerBackEnd) UnsealSector(ctx context.Context, sectorID uint64) (io.ReadCloser, error) {
	info, err := s.miner.GetSectorInfo(abi.SectorNumber(sectorID))
	if err != nil {
		return nil, errors.Errorf("failed to find sector info for sector with id: %d", sectorID)
	}

	// moving back to SDR means that we will no longer support partial unsealing
	return s.builder.ReadPieceFromSealedSector(ctx, abi.SectorNumber(sectorID), 0, abi.PaddedPieceSize(s.builder.SectorSize()).Unpadded(), info.Ticket.TicketBytes, info.CommD)
}

// LocatePieceForDealWithinSector uses the chain to locate an on-chain deal's
// piece within a sealed sector, producing an error if the provided deal does
// not exist on-chain, or if the sector into which the piece was written has
// not been pre-committed to the network.
func (s *StorageMinerBackEnd) LocatePieceForDealWithinSector(ctx context.Context, dealID uint64) (sectorID uint64, offset uint64, length uint64, err error) {
	sectors, err := s.miner.ListSectors()
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to list sectors")
	}

	isEncoded := func(s storage.SectorState) bool {
		return s == storage.Proving || s == storage.FinalizeSector || s == storage.CommitWait || s == storage.Committing || s == storage.WaitSeed
	}

	for _, sector := range sectors {
		offset := uint64(0)
		for _, piece := range sector.Pieces {
			if piece.DealInfo.DealID == abi.DealID(dealID) {
				if !isEncoded(sector.State) {
					return 0, 0, 0, errors.Errorf("no encoded replica exists corresponding to deal id: %d", dealID)
				}

				return uint64(sector.SectorNum), offset, uint64(piece.Piece.Size.Unpadded()), nil
			}

			offset += uint64(piece.Piece.Size.Unpadded())
		}
	}

	return 0, 0, 0, errors.Errorf("no encoded piece could be found corresponding to deal id: %d", dealID)
}
