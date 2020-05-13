package piecemanager

import (
	"context"
	"io"

	"github.com/filecoin-project/go-address"
	sectorstorage "github.com/filecoin-project/sector-storage"
	"github.com/filecoin-project/sector-storage/ffiwrapper"
	"github.com/filecoin-project/specs-actors/actors/abi"
	fsm "github.com/filecoin-project/storage-fsm"
	"github.com/pkg/errors"
)

var _ PieceManager = new(FiniteStateMachineBackEnd)

type FiniteStateMachineBackEnd struct {
	idc fsm.SectorIDCounter
	fsm *fsm.Sealing
	mgr sectorstorage.SectorManager
	mad address.Address
	ssz abi.SectorSize
}

func NewFiniteStateMachineBackEnd(mad address.Address, ssz abi.SectorSize, fsm *fsm.Sealing, mgr sectorstorage.SectorManager, idc fsm.SectorIDCounter) FiniteStateMachineBackEnd {
	return FiniteStateMachineBackEnd{
		fsm: fsm,
		idc: idc,
		mad: mad,
		mgr: mgr,
		ssz: ssz,
	}
}

func (f *FiniteStateMachineBackEnd) SealPieceIntoNewSector(ctx context.Context, dealID abi.DealID, dealStart, dealEnd abi.ChainEpoch, pieceSize abi.UnpaddedPieceSize, pieceReader io.Reader) error {
	sectorNumber, err := f.idc.Next()
	if err != nil {
		return err
	}

	return f.fsm.SealPiece(ctx, pieceSize, pieceReader, sectorNumber, fsm.DealInfo{
		DealID: dealID,
		DealSchedule: fsm.DealSchedule{
			StartEpoch: dealStart,
			EndEpoch:   dealEnd,
		},
	})
}

func (f *FiniteStateMachineBackEnd) PledgeSector(ctx context.Context) error {
	return f.fsm.PledgeSector()
}

func (f *FiniteStateMachineBackEnd) UnsealSector(ctx context.Context, sectorNumber uint64) (io.ReadCloser, error) {
	sn := abi.SectorNumber(sectorNumber)

	info, err := f.fsm.GetSectorInfo(sn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get sector info from FSM for sector unseal")
	}

	minerID, err := address.IDFromAddress(f.mad)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert miner address to ID for sector unseal")
	}

	sectorID := abi.SectorID{
		Miner:  abi.ActorID(minerID),
		Number: sn,
	}

	sectorSizeUnpadded := abi.PaddedPieceSize(f.ssz).Unpadded()

	return f.mgr.ReadPieceFromSealedSector(ctx, sectorID, ffiwrapper.UnpaddedByteIndex(0), sectorSizeUnpadded, info.TicketValue, *info.CommD)
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
