package market

import (
	market7 "github.com/filecoin-project/specs-actors/v7/actors/builtin/market"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/market"
)

type PieceStatus string

const (
	Undefine PieceStatus = "Undefine"
	Assigned PieceStatus = "Assigned"
	Packing  PieceStatus = "Packing"
	Proving  PieceStatus = "Proving"
)

type DealInfo struct {
	piecestore.DealInfo
	market.ClientDealProposal

	TransferType  string
	Root          cid.Cid
	PublishCid    cid.Cid
	FastRetrieval bool
	Status        PieceStatus
}

type GetDealSpec struct {
	// max limit of deal count
	MaxPiece int

	// max limit of date size in one single deal
	MaxPieceSize uint64

	// min limit of deal count
	MinPiece int

	// min limit of data size in one single deal
	MinPieceSize uint64

	// min limit of total space used by deals
	MinUsedSpace uint64
}

type DealInfoIncludePath struct {
	market7.DealProposal
	Offset          abi.PaddedPieceSize
	Length          abi.PaddedPieceSize
	PayloadSize     uint64
	DealID          abi.DealID
	TotalStorageFee abi.TokenAmount
	FastRetrieval   bool
	PublishCid      cid.Cid
}

type PieceInfo struct {
	PieceCID cid.Cid
	Deals    []*DealInfo
}
