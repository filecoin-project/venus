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

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/types"
)

type NodeClient struct {
	api *NodeAPI
}

func NewNodeClient(api *NodeAPI) *NodeClient {
	return &NodeClient{api: api}
}

func (api *NodeClient) AddBid(ctx context.Context, fromAddr types.Address, size *types.BytesAmount, price *types.AttoFIL) (*cid.Cid, error) {
	nd := api.api.node

	if err := setDefaultFromAddr(&fromAddr, nd); err != nil {
		return nil, err
	}

	funds := price.CalculatePrice(size)
	params, err := abi.ToEncodedValues(price, size)
	if err != nil {
		return nil, err
	}

	msg, err := node.NewMessageWithNextNonce(ctx, nd, fromAddr, address.StorageMarketAddress, funds, "addBid", params)
	if err != nil {
		return nil, err
	}

	smsg, err := types.NewSignedMessage(*msg, nd.Wallet)
	if err != nil {
		return nil, err
	}

	if err := nd.AddNewMessage(ctx, smsg); err != nil {
		return nil, err
	}

	return smsg.Cid()
}

func (api *NodeClient) Cat(ctx context.Context, c *cid.Cid) (uio.DagReader, error) {
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

func (api *NodeClient) ProposeDeal(ctx context.Context, askID, bidID uint, c *cid.Cid) (*node.DealResponse, error) {
	nd := api.api.node
	defaddr := nd.RewardAddress()

	propose := &node.DealProposal{
		ClientSig: string(defaddr[:]), // TODO: actual crypto
		Deal: &storagemarket.Deal{
			Ask:     uint64(askID),
			Bid:     uint64(bidID),
			DataRef: c.String(),
		},
	}

	return nd.StorageClient.ProposeDeal(ctx, propose)
}

func (api *NodeClient) QueryDeal(ctx context.Context, idSlice []byte) (*node.DealResponse, error) {
	if len(idSlice) != 32 {
		return nil, errors.New("id must be 32 bytes long")
	}

	var id [32]byte
	copy(id[:], idSlice)

	return api.api.node.StorageClient.QueryDeal(ctx, id)
}

func (api *NodeClient) ImportData(ctx context.Context, data io.Reader) (ipld.Node, error) {
	ds := dag.NewDAGService(api.api.node.Blockservice)
	spl := chunk.DefaultSplitter(data)

	return imp.BuildDagFromReader(ds, spl)
}
