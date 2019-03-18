package api

import (
	"context"

	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
)

// Client is the interface that defines methods to manage client operations.
type Client interface {
	ProposeStorageDeal(ctx context.Context, data cid.Cid, miner address.Address, ask uint64, duration uint64, allowDuplicates bool) (*storagedeal.Response, error)
	QueryStorageDeal(ctx context.Context, prop cid.Cid) (*storagedeal.Response, error)
	Payments(ctx context.Context, dealCid cid.Cid) ([]*paymentbroker.PaymentVoucher, error)
}
