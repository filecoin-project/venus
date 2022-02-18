package market

import (
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-state-types/abi"
	market7 "github.com/filecoin-project/specs-actors/v7/actors/builtin/market"

	"github.com/filecoin-project/venus/venus-shared/actors/builtin/market"
)

const (
	Undefine = "Undefine"
	Assigned = "Assigned"
	Packing  = "Packing"
	Proving  = "Proving"
)

type DealInfo struct {
	piecestore.DealInfo
	market.ClientDealProposal

	TransferType  string
	Root          cid.Cid
	PublishCid    cid.Cid
	FastRetrieval bool
	Status        string
}

type GetDealSpec struct {
	MaxPiece     int
	MaxPieceSize uint64
}

type DealInfoIncludePath struct {
	Offset          abi.PaddedPieceSize
	Length          abi.PaddedPieceSize
	PayloadSize     abi.UnpaddedPieceSize
	DealID          abi.DealID
	TotalStorageFee abi.TokenAmount
	market7.DealProposal
	FastRetrieval bool
	PublishCid    cid.Cid
}

type PieceInfo struct {
	PieceCID cid.Cid
	Deals    []*DealInfo
}
