package storage

import (
	"context"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	"github.com/filecoin-project/go-filecoin/types"
)

// API here is the API for a storage client.
type API struct {
	sc *Client
}

// NewAPI creates a new API for a storage client.
func NewAPI(storageClient *Client) API {
	return API{sc: storageClient}
}

// ProposeStorageDeal calls the storage client ProposeDeal function
func (a *API) ProposeStorageDeal(ctx context.Context, data cid.Cid, miner address.Address,
	askid uint64, duration uint64, allowDuplicates bool) (*storagedeal.Response, error) {

	return a.sc.ProposeDeal(ctx, miner, data, askid, duration, allowDuplicates)
}

// QueryStorageDeal calls the storage client QueryDeal function
func (a *API) QueryStorageDeal(ctx context.Context, prop cid.Cid) (*storagedeal.Response, error) {
	return a.sc.QueryDeal(ctx, prop)
}

// Payments calls the storage client LoadVouchersForDeal function
func (a *API) Payments(ctx context.Context, dealCid cid.Cid) ([]*types.PaymentVoucher, error) {
	return a.sc.LoadVouchersForDeal(ctx, dealCid)
}
