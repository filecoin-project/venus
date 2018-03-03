package core

import (
	"math/big"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/types"
)

func init() {
	cbor.RegisterCborType(Orderbook{})
}

// Orderbook holds all the bids and asks
type Orderbook struct {
	// Asks is the set of live asks in the orderbook
	Asks AskSet
	// NextAskID is the ID that will be assigned to the next ask that is created
	NextAskID uint64

	Bids      BidSet
	NextBidID uint64
}

// Ask is a storage market ask order.
type Ask struct {
	Price *big.Int
	Size  *big.Int
	Owner types.Address
	ID    uint64
}

// Bid is a storage market bid order.
type Bid struct {
	//Expiry *big.Int
	Price *big.Int
	Size  *big.Int
	//Duration *big.Int
	Collateral *big.Int
	//Coding ???
	Owner types.Address
	ID    uint64
}
