package impl

import (
	"context"

	"github.com/filecoin-project/go-filecoin/api"

	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
)

type nodeClient struct {
	api *nodeAPI
}

func newNodeClient(api *nodeAPI) *nodeClient {
	return &nodeClient{api: api}
}

var _ api.Client = &nodeClient{}

func (api *nodeClient) ProposeStorageDeal(ctx context.Context, data cid.Cid, miner address.Address, askid uint64, duration uint64, allowDuplicates bool) (*storagedeal.Response, error) {
	return api.api.node.StorageMinerClient.ProposeDeal(ctx, miner, data, askid, duration, allowDuplicates)
}

func (api *nodeClient) QueryStorageDeal(ctx context.Context, prop cid.Cid) (*storagedeal.Response, error) {
	return api.api.node.StorageMinerClient.QueryDeal(ctx, prop)
}

func (api *nodeClient) Payments(ctx context.Context, dealCid cid.Cid) ([]*paymentbroker.PaymentVoucher, error) {
	return api.api.node.StorageMinerClient.LoadVouchersForDeal(dealCid)
}
