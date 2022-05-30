package market

import (
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/go-state-types/builtin/v8/market"
)

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
		Ref:                   deal.Ref,
		AvailableForRetrieval: deal.AvailableForRetrieval,

		DealID:       deal.DealID,
		CreationTime: deal.CreationTime,

		TransferChannelId: deal.TransferChannelID,
		SectorNumber:      deal.SectorNumber,

		InboundCAR: deal.InboundCAR,
	}
}
