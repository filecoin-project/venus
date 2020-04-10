package piecemanager

import (
	"context"
	"io"

	"github.com/filecoin-project/go-address"
	sectorstorage "github.com/filecoin-project/sector-storage"
	"github.com/filecoin-project/specs-actors/actors/abi"
	fsm "github.com/filecoin-project/storage-fsm"
)

type Foo struct {
	maddr address.Address
	sid   fsm.SectorIDCounter
	mgr   sectorstorage.SectorManager
}

func (f *Foo) SealPieceIntoNewSector(ctx context.Context, dealID abi.DealID, dealStart, dealEnd abi.ChainEpoch, pieceSize abi.UnpaddedPieceSize, pieceReader io.Reader) error {
	sectorNumber, err := f.sid.Next()
	if err != nil {
		return err
	}

	minerID, err := address.IDFromAddress(f.maddr)
	if err != nil {
		return err
	}

	sectorID := abi.SectorID{
		Miner:  abi.ActorID(minerID),
		Number: sectorNumber,
	}

	err = f.mgr.NewSector(ctx, sectorID)
	if err != nil {
		return err
	}

	return nil
}

func (f *Foo) PledgeSector(ctx context.Context) error {
	panic("implement me")
}

func (f *Foo) UnsealSector(ctx context.Context, sectorID uint64) (io.ReadCloser, error) {
	panic("implement me")
}

func (f *Foo) LocatePieceForDealWithinSector(ctx context.Context, dealID uint64) (sectorID uint64, offset uint64, length uint64, err error) {
	panic("implement me")
}

var _ PieceManager = new(Foo)
