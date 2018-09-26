package api

import (
	"context"
	"io"

	ipld "gx/ipfs/QmX5CsuHyVZeTLxgRSYkgLSDQKb9UjE8xnhQzCEJWWWFsC/go-ipld-format"
	cid "gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	uio "gx/ipfs/Qmdg2crJzNUF1mLPnLPSCCaDdLDqE4Qrh9QEiDooSYkvuB/go-unixfs/io"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/types"
)

// Client is the interface that defines methods to manage client operations.
type Client interface {
	AddBid(ctx context.Context, fromAddr address.Address, size *types.BytesAmount, price *types.AttoFIL) (*cid.Cid, error)
	Cat(ctx context.Context, c *cid.Cid) (uio.DagReader, error)
	ProposeDeal(ctx context.Context, fromAddr address.Address, askID, bidID uint, c *cid.Cid) (*node.DealResponse, error)
	QueryDeal(ctx context.Context, idSlice []byte) (*node.DealResponse, error)
	ImportData(ctx context.Context, data io.Reader) (ipld.Node, error)
	ProposeStorageDeal(ctx context.Context, data *cid.Cid, miner address.Address, price *types.AttoFIL, duration uint64) (*cid.Cid, error)
	QueryStorageDeal(ctx context.Context, prop *cid.Cid) (*node.StorageDealResponse, error)
}
