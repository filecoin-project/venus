package node_api

import (
	"context"

	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"

	"github.com/filecoin-project/go-filecoin/types"
)

type BlockAPI struct {
	api *API
}

func NewBlockAPI(api *API) *BlockAPI {
	return &BlockAPI{api: api}
}

func (api *BlockAPI) Get(ctx context.Context, id *cid.Cid) (*types.Block, error) {
	return api.api.node.ChainMgr.FetchBlock(ctx, id)
}
