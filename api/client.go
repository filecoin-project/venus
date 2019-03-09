package api

import (
	"context"
	"io"

	cid "github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	uio "github.com/ipfs/go-unixfs/io"

	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	"github.com/filecoin-project/go-filecoin/types"
)

// Ask is a result of querying for an ask, it may contain an error
type Ask struct {
	Miner  address.Address
	Price  *types.AttoFIL
	Expiry *types.BlockHeight
	ID     uint64

	Error error
}

// Client is the interface that defines methods to manage client operations.
type Client interface {
	Cat(ctx context.Context, c cid.Cid) (uio.DagReader, error)
	ImportData(ctx context.Context, data io.Reader) (ipld.Node, error)
	ProposeStorageDeal(ctx context.Context, data cid.Cid, miner address.Address, ask uint64, duration uint64, allowDuplicates bool) (*storagedeal.Response, error)
	QueryStorageDeal(ctx context.Context, prop cid.Cid) (*storagedeal.Response, error)
	ListAsks(ctx context.Context) (<-chan Ask, error)
	Payments(ctx context.Context, dealCid cid.Cid) ([]*paymentbroker.PaymentVoucher, error)
}
