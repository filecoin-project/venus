package impl

import (
	"context"
	"io"

	chunk "gx/ipfs/QmVDjhUMtkRskBFAVNwyXuLSKbeAya7JKPnzAxMKDaK4x4/go-ipfs-chunker"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	imp "gx/ipfs/QmXBooHftCHoCUmwuxSibWCgLzmRw2gd2FBTJowsWKy9vE/go-unixfs/importer"
	uio "gx/ipfs/QmXBooHftCHoCUmwuxSibWCgLzmRw2gd2FBTJowsWKy9vE/go-unixfs/io"
	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	ipld "gx/ipfs/QmZtNq8dArGfnpCZfx2pUNY7UcjGhVp5qqwQ4hH6mpTMRQ/go-ipld-format"
	dag "gx/ipfs/QmeCaeBmCCEJrZahwXY4G2G8zRaNBWskrfKWoQ6Xv6c1DR/go-merkledag"

	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/types"
)

type nodeClient struct {
	api *nodeAPI
}

func newNodeClient(api *nodeAPI) *nodeClient {
	return &nodeClient{api: api}
}

func (api *nodeClient) AddBid(ctx context.Context, fromAddr types.Address, size *types.BytesAmount, price *types.AttoFIL) (*cid.Cid, error) {
	funds := price.CalculatePrice(size)

	return api.api.Message().Send(
		ctx,
		fromAddr,
		address.StorageMarketAddress,
		funds,
		"addBid",
		price, size,
	)
}

func (api *nodeClient) Cat(ctx context.Context, c *cid.Cid) (uio.DagReader, error) {
	// TODO: this goes back to 'how is data stored and referenced'
	// For now, lets just do things the ipfs way.

	nd := api.api.node
	ds := dag.NewDAGService(nd.Blockservice)

	data, err := ds.Get(ctx, c)
	if err != nil {
		return nil, err
	}

	return uio.NewDagReader(ctx, data, ds)
}

func (api *nodeClient) ProposeDeal(ctx context.Context, askID, bidID uint, c *cid.Cid) (*node.DealResponse, error) {
	nd := api.api.node
	defaddr, err := nd.DefaultSenderAddress()
	if err != nil {
		return nil, err
	}

	deal := &storagemarket.Deal{
		Ask:     uint64(askID),
		Bid:     uint64(bidID),
		DataRef: c.String(),
	}

	propose, err := node.NewDealProposal(deal, nd.Wallet, defaddr)
	if err != nil {
		return nil, err
	}

	return nd.StorageClient.ProposeDeal(ctx, propose)
}

func (api *nodeClient) QueryDeal(ctx context.Context, idSlice []byte) (*node.DealResponse, error) {
	if len(idSlice) != 32 {
		return nil, errors.New("id must be 32 bytes long")
	}

	var id [32]byte
	copy(id[:], idSlice)

	return api.api.node.StorageClient.QueryDeal(ctx, id)
}

func (api *nodeClient) ImportData(ctx context.Context, data io.Reader) (ipld.Node, error) {
	ds := dag.NewDAGService(api.api.node.Blockservice)
	spl := chunk.DefaultSplitter(data)

	return imp.BuildDagFromReader(ds, spl)
}

func (api *nodeClient) ProposeStorageDeal(ctx context.Context, data *cid.Cid, miner types.Address, price *types.AttoFIL, duration uint64) (*cid.Cid, error) {
	return api.api.node.StorageMinerClient.TryToStoreData(ctx, miner, data, duration, price)
}

func (api *nodeClient) QueryStorageDeal(ctx context.Context, prop *cid.Cid) (*node.StorageDealResponse, error) {
	return api.api.node.StorageMinerClient.Query(ctx, prop)
}
