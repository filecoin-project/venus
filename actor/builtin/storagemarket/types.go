package storagemarket

import (
	"math/big"

	cbor "gx/ipfs/QmPbqRavwDZLfmpeW6eoyAoQ5rT2LoCW98JhvRc22CqkZS/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/types"
)

func init() {
	cbor.RegisterCborType(Orderbook{})
	cbor.RegisterCborType(Deal{})
}

// Orderbook holds all the bids and asks
type Orderbook struct {
	// Asks is the set of live asks in the orderbook
	StorageAsks AskSet
	// NextSAskID is the ID that will be assigned to the next ask that is created
	NextSAskID uint64

	Bids      BidSet
	NextBidID uint64
}

// Ask is a storage market ask order.
type Ask struct {
	Price *types.AttoFIL     `json:"price":` // nolint vet
	Size  *types.BytesAmount `json:"size"`
	Owner types.Address      `json:"owner"`
	ID    uint64             `json:"id"`
}

// Bid is a storage market bid order.
type Bid struct {
	//Expiry *big.Int
	Price *types.AttoFIL     `json:"price"`
	Size  *types.BytesAmount `json:"size"`
	//Duration *big.Int
	Collateral *types.AttoFIL `json:"collateral"`
	//Coding ???
	Owner types.Address `json:"owner"`
	ID    uint64        `json:"id"`

	// Used indicates whether or not this bid is in use by a deal
	Used bool `json:"used"`
}

// Deal is the successful fulfilment of an ask and a bid with eachother.
type Deal struct {
	Expiry  *big.Int `json:"expiry"`
	DataRef string   `json:"dataRef"`

	Ask uint64 `json:"ask"`
	Bid uint64 `json:"bid"`
}
