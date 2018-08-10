package impl

import (
	"context"

	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"

	"github.com/filecoin-project/go-filecoin/types"
)

type nodeBlock struct {
	api *nodeAPI
}

func newNodeBlock(api *nodeAPI) *nodeBlock {
	return &nodeBlock{api: api}
}

func (api *nodeBlock) Get(ctx context.Context, id *cid.Cid) (*types.Block, error) {
	return api.api.node.ChainMgr.FetchBlock(ctx, id)
}
