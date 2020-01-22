package storage

import (
	"context"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/protocol/storage/storagedeal"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
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
	askid uint64, duration uint64, allowDuplicates bool) (*storagedeal.SignedResponse, error) {

	return a.sc.ProposeDeal(ctx, miner, data, askid, duration, allowDuplicates)
}

// QueryStorageDeal calls the storage client QueryDeal function
func (a *API) QueryStorageDeal(ctx context.Context, prop cid.Cid) (*storagedeal.SignedResponse, error) {
	panic("storage client integration will change during go-fil-markets integration")

	return nil, nil // nolint:govet
}

// Payments calls the storage client LoadVouchersForDeal function
func (a *API) Payments(ctx context.Context, dealCid cid.Cid) ([]*types.PaymentVoucher, error) {
	panic("storage client integration will change during go-fil-markets integration")

	return nil, nil // nolint:govet
}
