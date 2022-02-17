package client

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/venus-shared/types"
)

type RetrievalOrder struct {
	// TODO: make this less unixfs specific
	Root         cid.Cid
	Piece        *cid.Cid
	DataSelector *DataSelector

	Size  uint64
	Total types.BigInt

	UnsealPrice             types.BigInt
	PaymentInterval         uint64
	PaymentIntervalIncrease uint64
	Client                  address.Address
	Miner                   address.Address
	MinerPeer               *retrievalmarket.RetrievalPeer
}

type RestrievalRes struct {
	DealID retrievalmarket.DealID
}
