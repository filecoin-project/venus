package impl

import (
	"context"

	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"

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
