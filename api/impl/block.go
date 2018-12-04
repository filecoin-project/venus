package impl

import (
	"context"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/types"
)

type nodeBlock struct {
	api *nodeAPI
}

func newNodeBlock(api *nodeAPI) *nodeBlock {
	return &nodeBlock{api: api}
}

func (api *nodeBlock) Get(ctx context.Context, id *cid.Cid) (*types.Block, error) {
	return api.api.node.ChainReader.GetBlock(ctx, id)
}
