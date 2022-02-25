package client

import (
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/venus/venus-shared/types/market"
)

type RetrievalInfo struct {
	PayloadCID   cid.Cid
	ID           retrievalmarket.DealID
	PieceCID     *cid.Cid
	PricePerByte abi.TokenAmount
	UnsealPrice  abi.TokenAmount

	Status        retrievalmarket.DealStatus
	Message       string // more information about deal state, particularly errors
	Provider      peer.ID
	BytesReceived uint64
	BytesPaidFor  uint64
	TotalPaid     abi.TokenAmount

	TransferChannelID *datatransfer.ChannelID
	DataTransfer      *market.DataTransferChannel

	// optional event if part of ClientGetRetrievalUpdates
	Event *retrievalmarket.ClientEvent
}
