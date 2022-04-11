package market

import (
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/venus/venus-shared/actors/builtin/market"
	"github.com/filecoin-project/venus/venus-shared/types"
)

type MinerDealV0 struct {
	market.ClientDealProposal
	ProposalCid           cid.Cid
	AddFundsCid           *cid.Cid
	PublishCid            *cid.Cid
	Miner                 peer.ID
	Client                peer.ID
	State                 storagemarket.StorageDealStatus
	PiecePath             filestore.Path
	PayloadSize           uint64
	MetadataPath          filestore.Path
	SlashEpoch            abi.ChainEpoch
	FastRetrieval         bool
	Message               string
	FundsReserved         abi.TokenAmount
	Ref                   *storagemarket.DataRef
	AvailableForRetrieval bool

	DealID       abi.DealID
	CreationTime cbg.CborTime

	TransferChannelID *datatransfer.ChannelID `json:"TransferChannelId"`
	SectorNumber      abi.SectorNumber

	Offset      abi.PaddedPieceSize
	PieceStatus PieceStatus

	InboundCAR string
}

type MinerDeal struct {
	market.ClientDealProposal
	ProposalCid           cid.Cid
	AddFundsCid           *cid.Cid
	PublishCid            *cid.Cid
	Miner                 peer.ID
	Client                peer.ID
	State                 storagemarket.StorageDealStatus
	PiecePath             filestore.Path
	PayloadSize           uint64
	MetadataPath          filestore.Path
	SlashEpoch            abi.ChainEpoch
	FastRetrieval         bool
	Message               string
	FundsReserved         abi.TokenAmount
	Ref                   *DataRef
	AvailableForRetrieval bool

	DealID       abi.DealID
	CreationTime cbg.CborTime

	TransferChannelID *datatransfer.ChannelID `json:"TransferChannelId"`
	SectorNumber      abi.SectorNumber

	Offset      abi.PaddedPieceSize
	PieceStatus PieceStatus

	InboundCAR string
}

// DataRef is a reference for how data will be transferred for a given storage deal
type DataRef struct {
	TransferType string // include: graphsync, manual, import, http
	Root         cid.Cid

	Params   []byte // Params include http url and headers, when TransferType is `http`
	State    int64
	DealUUID types.UUID

	PieceCid     *cid.Cid              // Optional for non-manual transfer, will be recomputed from the data if not given
	PieceSize    abi.UnpaddedPieceSize // Optional for non-manual transfer, will be recomputed from the data if not given
	RawBlockSize uint64                // Optional: used as the denominator when calculating transfer %
}

func (deal *MinerDeal) FilMarketMinerDeal() *storagemarket.MinerDeal {
	return &storagemarket.MinerDeal{
		ClientDealProposal:    deal.ClientDealProposal,
		ProposalCid:           deal.ProposalCid,
		AddFundsCid:           deal.AddFundsCid,
		PublishCid:            deal.PublishCid,
		Miner:                 deal.Miner,
		Client:                deal.Client,
		State:                 deal.State,
		PiecePath:             deal.PiecePath,
		MetadataPath:          deal.MetadataPath,
		SlashEpoch:            deal.SlashEpoch,
		FastRetrieval:         deal.FastRetrieval,
		Message:               deal.Message,
		FundsReserved:         deal.FundsReserved,
		Ref:                   deal.FillDataRef(),
		AvailableForRetrieval: deal.AvailableForRetrieval,

		DealID:       deal.DealID,
		CreationTime: deal.CreationTime,

		TransferChannelId: deal.TransferChannelID,
		SectorNumber:      deal.SectorNumber,

		InboundCAR: deal.InboundCAR,
	}
}

func (deal *MinerDeal) FillDataRef() *storagemarket.DataRef {
	return &storagemarket.DataRef{
		TransferType: deal.Ref.TransferType,
		Root:         deal.Ref.Root,
		PieceCid:     deal.Ref.PieceCid,
		PieceSize:    deal.Ref.PieceSize,
		RawBlockSize: deal.Ref.RawBlockSize,
	}
}

func FillDataRef(ref *storagemarket.DataRef) *DataRef {
	return &DataRef{
		TransferType: ref.TransferType,
		Root:         ref.Root,
		PieceCid:     ref.PieceCid,
		PieceSize:    ref.PieceSize,
		RawBlockSize: ref.RawBlockSize,
	}
}
