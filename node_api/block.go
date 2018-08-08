package node_api

import (
	"context"

	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"

	"github.com/filecoin-project/go-filecoin/types"
)

type NodeBlock struct {
	api *API
}

func NewNodeBlock(api *API) *NodeBlock {
	return &NodeBlock{api: api}
}

func (api *NodeBlock) Get(ctx context.Context, id *cid.Cid) (*types.Block, error) {
	return api.api.node.ChainMgr.FetchBlock(ctx, id)
}
