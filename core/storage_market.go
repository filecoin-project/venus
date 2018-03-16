package core

import (
	"math/big"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/filecoin-project/go-filecoin/types"
)

func init() {
	cbor.RegisterCborType(Orderbook{})
	cbor.RegisterCborType(Deal{})
}

// Orderbook holds all the bids and asks
type Orderbook struct {
	// Asks is the set of live asks in the orderbook
	Asks AskSet
	// NextAskID is the ID that will be assigned to the next ask that is created
	NextAskID uint64

	Bids      BidSet
	NextBidID uint64

	Deals []*Deal
}

// Ask is a storage market ask order.
type Ask struct {
	Price *types.TokenAmount
	Size  *types.BytesAmount
	Owner types.Address
	ID    uint64
}

// Bid is a storage market bid order.
type Bid struct {
	//Expiry *big.Int
	Price *types.TokenAmount
	Size  *types.BytesAmount
	//Duration *big.Int
	Collateral *types.TokenAmount
	//Coding ???
	Owner types.Address
	ID    uint64

	// Used indicates whether or not this bid is in use by a deal
	Used bool
}

// Deal is the successful fulfilment of an ask and a bid with eachother.
type Deal struct {
	Expiry  *big.Int
	DataRef *cid.Cid

	Ask uint64
	Bid uint64
}
