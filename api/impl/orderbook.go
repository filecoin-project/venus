package impl

import "github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"

type nodeOrderbook struct {
	api *nodeAPI
}

func newNodeOrderbook(api *nodeAPI) *nodeOrderbook {
	return &nodeOrderbook{api: api}
}

func (api *nodeOrderbook) Asks() (storagemarket.AskSet, error) {
	return api.api.node.StorageMarket.GetMarketPeeker().GetAskSet()
}
func (api *nodeOrderbook) Bids() (storagemarket.BidSet, error) {
	return api.api.node.StorageMarket.GetMarketPeeker().GetBidSet()
}
func (api *nodeOrderbook) Deals() ([]*storagemarket.Deal, error) {
	return api.api.node.StorageMarket.GetMarketPeeker().GetDealList()
}
