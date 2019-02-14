package api

import (
	"context"
	"io"

	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	ipld "gx/ipfs/QmRL22E4paat7ky7vx9MLpR97JHHbFPrg3ytFQw6qp1y1s/go-ipld-format"
	uio "gx/ipfs/QmSygPSC63Uka8z9PYokAS4thiMAor17vhXUTi4qmKHh6P/go-unixfs/io"

	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/protocol/storage"
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
	ProposeStorageDeal(ctx context.Context, data cid.Cid, miner address.Address, ask uint64, duration uint64, allowDuplicates bool) (*storage.DealResponse, error)
	QueryStorageDeal(ctx context.Context, prop cid.Cid) (*storage.DealResponse, error)
	ListAsks(ctx context.Context) (<-chan Ask, error)
	Payments(ctx context.Context, dealCid cid.Cid) ([]*paymentbroker.PaymentVoucher, error)
}
