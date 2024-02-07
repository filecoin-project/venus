package market

import (
	"github.com/ipfs/go-cid"

	address "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/venus-shared/types"
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
	types.ClientDealProposal

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

	// start epoch limit of the chosen deals
	// if set, the deals should not be activated before or equal than the this epoch
	StartEpoch abi.ChainEpoch

	// end epoch limit of the chosen deals
	// if set, the deals should not be alive after or equal than the this epoch
	EndEpoch abi.ChainEpoch

	// Filter by sector lifetime, for snapdeal
	SectorExpiration *abi.ChainEpoch
}

type DealInfoIncludePath struct {
	types.DealProposal
	Offset          abi.PaddedPieceSize
	Length          abi.PaddedPieceSize
	PayloadSize     uint64
	DealID          abi.DealID
	TotalStorageFee abi.TokenAmount
	FastRetrieval   bool
	PublishCid      cid.Cid
}

type DealInfoV2 struct {
	DealID     abi.DealID
	PublishCid cid.Cid

	AllocationID types.AllocationId

	PieceCID    cid.Cid
	PieceSize   abi.PaddedPieceSize
	Client      address.Address
	Provider    address.Address
	Offset      abi.PaddedPieceSize
	Length      abi.PaddedPieceSize
	PayloadSize uint64
	StartEpoch  abi.ChainEpoch
	EndEpoch    abi.ChainEpoch
}

func (d *DealInfoV2) IsBuiltinMarket() bool {
	return d.PublishCid.Defined()
}

type DirectDealInfo struct {
	AllocationID types.AllocationId
	PieceCID     cid.Cid
	PieceSize    abi.PaddedPieceSize
	Client       address.Address
	Provider     address.Address
	Offset       abi.PaddedPieceSize
	Length       abi.PaddedPieceSize
	PayloadSize  uint64
	StartEpoch   abi.ChainEpoch
	EndEpoch     abi.ChainEpoch
}

type PieceInfo struct {
	PieceCID cid.Cid
	Deals    []*DealInfo
}

type TimeStamp struct {
	CreatedAt uint64
	UpdatedAt uint64
}
