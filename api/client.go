package api

import (
	"context"
	"io"

	uio "gx/ipfs/QmXBooHftCHoCUmwuxSibWCgLzmRw2gd2FBTJowsWKy9vE/go-unixfs/io"
	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	ipld "gx/ipfs/QmZtNq8dArGfnpCZfx2pUNY7UcjGhVp5qqwQ4hH6mpTMRQ/go-ipld-format"

	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/types"
)

// Client is the interface that defines methods to manage client operations.
type Client interface {
	AddBid(ctx context.Context, fromAddr types.Address, size *types.BytesAmount, price *types.AttoFIL) (*cid.Cid, error)
	Cat(ctx context.Context, c *cid.Cid) (uio.DagReader, error)
	ProposeDeal(ctx context.Context, askID, bidID uint, c *cid.Cid) (*node.DealResponse, error)
	QueryDeal(ctx context.Context, idSlice []byte) (*node.DealResponse, error)
	ImportData(ctx context.Context, data io.Reader) (ipld.Node, error)
}
