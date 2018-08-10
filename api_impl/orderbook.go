package api_impl

import "github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"

type NodeOrderbook struct {
	api *API
}

func NewNodeOrderbook(api *API) *NodeOrderbook {
	return &NodeOrderbook{api: api}
}

func (api *NodeOrderbook) Asks() (storagemarket.AskSet, error) {
	return api.api.node.StorageMarket.GetMarketPeeker().GetAskSet()
}
func (api *NodeOrderbook) Bids() (storagemarket.BidSet, error) {
	return api.api.node.StorageMarket.GetMarketPeeker().GetBidSet()
}
func (api *NodeOrderbook) Deals() ([]*storagemarket.Deal, error) {
	return api.api.node.StorageMarket.GetMarketPeeker().GetDealList()
}
