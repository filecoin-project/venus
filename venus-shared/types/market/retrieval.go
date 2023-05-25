package market

import (
	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer/v2"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
)

// ProviderDealState is the current state of a deal from the point of view
// of a retrieval provider
type ProviderDealState struct {
	retrievalmarket.DealProposal
	StoreID               uint64
	SelStorageProposalCid cid.Cid
	ChannelID             *datatransfer.ChannelID
	Status                retrievalmarket.DealStatus
	Receiver              peer.ID
	TotalSent             uint64
	FundsReceived         abi.TokenAmount
	Message               string
	CurrentInterval       uint64
	LegacyProtocol        bool
	TimeStamp
}

func (deal *ProviderDealState) TotalPaidFor() uint64 {
	totalPaidFor := uint64(0)
	if !deal.PricePerByte.IsZero() {
		totalPaidFor = big.Div(big.Max(big.Sub(deal.FundsReceived, deal.UnsealPrice), big.Zero()), deal.PricePerByte).Uint64()
	}
	return totalPaidFor
}

func (deal *ProviderDealState) IntervalLowerBound() uint64 {
	return deal.Params.IntervalLowerBound(deal.CurrentInterval)
}

func (deal *ProviderDealState) NextInterval() uint64 {
	return nextInterval(&deal.Params, deal.CurrentInterval)
}

func nextInterval(p *retrievalmarket.Params, currentInterval uint64) uint64 {
	intervalSize := p.PaymentInterval
	var nextInterval uint64
	for nextInterval <= currentInterval {
		nextInterval += intervalSize
		intervalSize += p.PaymentIntervalIncrease
	}
	return nextInterval
}

// Identifier provides a unique id for this provider deal
func (deal ProviderDealState) Identifier() retrievalmarket.ProviderDealIdentifier {
	return retrievalmarket.ProviderDealIdentifier{Receiver: deal.Receiver, DealID: deal.ID}
}

type RetrievalAsk struct {
	Miner                   address.Address
	PricePerByte            abi.TokenAmount
	UnsealPrice             abi.TokenAmount
	PaymentInterval         uint64
	PaymentIntervalIncrease uint64
	TimeStamp
}

type RetrievalDealQueryParams struct {
	Receiver          string
	PayloadCID        string
	Status            *uint64
	DiscardFailedDeal bool

	Page
}
