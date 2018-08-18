package api

import "github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"

// Orderbook is the interface that defines methods to interact with the orderbook.
type Orderbook interface {
	Asks() (storagemarket.AskSet, error)
	Bids() (storagemarket.BidSet, error)
	//Deals() ([]*storagemarket.Deal, error)
}
