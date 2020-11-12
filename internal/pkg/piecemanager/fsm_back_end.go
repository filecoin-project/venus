package piecemanager

import (
	"context"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"io"

	"github.com/pkg/errors"

	"github.com/filecoin-project/go-state-types/abi"
	fsm "github.com/filecoin-project/venus/vendors/storage-sealing"
)

var _ PieceManager = new(FiniteStateMachineBackEnd)

type FiniteStateMachineBackEnd struct {
	idc fsm.SectorIDCounter
	fsm *fsm.Sealing
}

func NewFiniteStateMachineBackEnd(fsm *fsm.Sealing, idc fsm.SectorIDCounter) FiniteStateMachineBackEnd {
	return FiniteStateMachineBackEnd{
		idc: idc,
		fsm: fsm,
	}
}

func (f *FiniteStateMachineBackEnd) SealPieceIntoNewSector(ctx context.Context, dealID abi.DealID, dealStart, dealEnd abi.ChainEpoch, pieceSize abi.UnpaddedPieceSize, pieceReader io.Reader) (*storagemarket.PackingResult, error) {
	sid, offset, err := f.fsm.AddPieceToAnySector(ctx, pieceSize, pieceReader, fsm.DealInfo{
		DealID: dealID,
		DealSchedule: fsm.DealSchedule{
			StartEpoch: dealStart,
			EndEpoch:   dealEnd,
		},
	})
	return &storagemarket.PackingResult{
		SectorNumber: sid,
		Offset:       offset,
		Size:         pieceSize.Padded(),
	}, err
}

func (f *FiniteStateMachineBackEnd) PledgeSector(ctx context.Context) error {
	return f.fsm.PledgeSector()
}

func (f *FiniteStateMachineBackEnd) UnsealSector(ctx context.Context, sectorID abi.SectorNumber) (io.ReadCloser, error) {
	panic("implement me")
}

func (f *FiniteStateMachineBackEnd) LocatePieceForDealWithinSector(ctx context.Context, dealID uint64) (sectorID uint64, offset uint64, length uint64, err error) {
	sectors, err := f.fsm.ListSectors()
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to list sectors")
	}

	isEncoded := func(s fsm.SectorState) bool {
		return fsm.PreCommit2 <= s && s <= fsm.Proving
	}

	for _, sector := range sectors {
		offset := uint64(0)
		for _, piece := range sector.Pieces {
			if piece.DealInfo.DealID == abi.DealID(dealID) {
				if !isEncoded(sector.State) {
					return 0, 0, 0, errors.Errorf("no encoded replica exists corresponding to deal id: %d", dealID)
				}

				return uint64(sector.SectorNumber), offset, uint64(piece.Piece.Size.Unpadded()), nil
			}

			offset += uint64(piece.Piece.Size.Unpadded())
		}
	}

	return 0, 0, 0, errors.Errorf("no encoded piece could be found corresponding to deal id: %d", dealID)
}
