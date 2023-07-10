package client

import (
	"time"

	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer/v2"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/types/market"
)

type DealParams struct {
	Data               *storagemarket.DataRef
	Wallet             address.Address
	Miner              address.Address
	EpochPrice         types.BigInt
	MinBlocksDuration  uint64
	ProviderCollateral types.BigInt
	DealStartEpoch     abi.ChainEpoch
	FastRetrieval      bool
	VerifiedDeal       bool
}

type DealInfo struct {
	ProposalCid cid.Cid
	State       storagemarket.StorageDealStatus
	Message     string // more information about deal state, particularly errors
	DealStages  *storagemarket.DealStages
	Provider    address.Address

	DataRef  *storagemarket.DataRef
	PieceCID cid.Cid
	Size     uint64

	PricePerEpoch types.BigInt
	Duration      uint64

	DealID abi.DealID

	CreationTime time.Time
	Verified     bool

	TransferChannelID *datatransfer.ChannelID
	DataTransfer      *market.DataTransferChannel
}

type DealResults struct {
	Results []*DealResult
}

type DealResult struct {
	ProposalCID cid.Cid
	// Create deal failed
	Message string
}

type ClientOfflineDeal struct { //nolint
	types.ClientDealProposal

	ProposalCID    cid.Cid
	DataRef        *storagemarket.DataRef
	Message        string
	State          uint64
	DealID         uint64
	AddFundsCid    *cid.Cid
	PublishMessage *cid.Cid
	FastRetrieval  bool
	SlashEpoch     abi.ChainEpoch
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (d *ClientOfflineDeal) DealInfo() *DealInfo {
	return &DealInfo{
		ProposalCid:   d.ProposalCID,
		DataRef:       d.DataRef,
		State:         d.State,
		Message:       d.Message,
		Provider:      d.Proposal.Provider,
		PieceCID:      d.Proposal.PieceCID,
		Size:          uint64(d.Proposal.PieceSize.Unpadded()),
		PricePerEpoch: d.Proposal.StoragePricePerEpoch,
		Duration:      uint64(d.Proposal.Duration()),
		DealID:        abi.DealID(d.DealID),
		CreationTime:  d.CreatedAt,
		Verified:      d.Proposal.VerifiedDeal,
	}
}

type ProviderDistribution struct {
	Provider address.Address
	// Total deal
	Total uint64
	// Uniq deal
	Uniq uint64
	// May be too large
	UniqPieces map[string]uint64
	// (Total-Uniq) / Total
	DuplicationPercentage float64
}

type ReplicaDistribution struct {
	// Datacap address
	Client address.Address
	// Total deal
	Total uint64
	// Uniq deal
	Uniq uint64
	// (Total-Uniq) / Uniq
	DuplicationPercentage float64
	// ProviderTotalDeal / Total
	ReplicasPercentage   map[string]float64
	ReplicasDistribution []*ProviderDistribution
}

type DealDistribution struct {
	ProvidersDistribution []*ProviderDistribution
	ReplicasDistribution  []*ReplicaDistribution
}
