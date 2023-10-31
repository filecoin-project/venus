package market

import (
	"time"

	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer/v2"
	"github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	crypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	cbg "github.com/whyrusleeping/cbor-gen"
)

type MinerDeal struct {
	ID uuid.UUID
	types.ClientDealProposal
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

	TimeStamp
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

// PendingDealInfo has info about pending deals and when they are due to be
// published
type PendingDealInfo struct {
	Deals              []types.ClientDealProposal
	PublishPeriodStart time.Time
	PublishPeriod      time.Duration
}

// StorageDealStatistic storage  statistical information
// The struct is used here for statistical information that may need to be added in the future
type StorageDealStatistic struct {
	DealsStatus map[storagemarket.StorageDealStatus]int64
}

// RetrievalDealStatistic storage  statistical information
// The struct is used here for statistical information that may need to be added in the future
type RetrievalDealStatistic struct {
	DealsStatus map[retrievalmarket.DealStatus]int64
}

// SignedStorageAsk use to record provider's requirement in database
type SignedStorageAsk struct {
	Ask       *storagemarket.StorageAsk
	Signature *crypto.Signature
	TimeStamp
}

func (sa *SignedStorageAsk) ToChainAsk() *storagemarket.SignedStorageAsk {
	return &storagemarket.SignedStorageAsk{
		Ask:       sa.Ask,
		Signature: sa.Signature,
	}
}

type Page struct {
	Offset int
	Limit  int
}

type StorageDealQueryParams struct {
	// provider
	Miner             address.Address
	State             *uint64
	Client            string
	DiscardFailedDeal bool

	DealID   abi.DealID
	PieceCID string

	Page
	Asc bool
}

type ImportDataRef struct {
	ProposalCID cid.Cid
	UUID        uuid.UUID
	File        string
}

type ImportDataRefs struct {
	Refs      []*ImportDataRef
	SkipCommP bool
}

type ImportDataResult struct {
	// Target may deal proposal cid or deal uuid
	Target string
	// deal import failed
	Message string
}
