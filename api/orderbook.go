package api

import "github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"

type Orderbook interface {
	Asks() (storagemarket.AskSet, error)
	Bids() (storagemarket.BidSet, error)
	Deals() ([]*storagemarket.Deal, error)
}
