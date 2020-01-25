package shared_testutils

import (
	"github.com/filecoin-project/go-fil-markets/piecestore"
)

type DummyPieceStore struct { }
func (d DummyPieceStore) AddDealForPiece(pieceCID []byte, dealInfo piecestore.DealInfo) error {
	panic("do not call. I'm a dummy")
}

func (d DummyPieceStore) AddBlockInfosToPiece(pieceCID []byte, blockInfos []piecestore.BlockInfo) error {
	panic("do not call. I'm a dummy")
}

func (d DummyPieceStore) HasBlockInfo(pieceCID []byte) (bool, error) {
	panic("do not call. I'm a dummy")
}

func (d DummyPieceStore) HasDealInfo(pieceCID []byte) (bool, error) {
	panic("do not call. I'm a dummy")
}

func (d DummyPieceStore) GetPieceInfo(pieceCID []byte) (piecestore.PieceInfo, error) {
	panic("do not call. I'm a dummy")
}


