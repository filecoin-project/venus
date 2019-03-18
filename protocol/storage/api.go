package storage

import (
	"context"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
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
func (a *API) Payments(ctx context.Context, dealCid cid.Cid) ([]*paymentbroker.PaymentVoucher, error) {
	return a.sc.LoadVouchersForDeal(dealCid)
}
