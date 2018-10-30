package impl

import (
	"context"
	"github.com/filecoin-project/go-filecoin/types"

	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
)

type nodeChain struct {
	api *nodeAPI
}

func newNodeChain(api *nodeAPI) *nodeChain {
	return &nodeChain{api: api}
}

func (api *nodeChain) Head() ([]*cid.Cid, error) {
	ts := api.api.node.ChainReader.Head()
	if len(ts) == 0 {
		return nil, ErrHeaviestTipSetNotFound
	}
	tsSlice := ts.ToSlice()
	out := types.SortedCidSet{}

	for _, b := range tsSlice {
		out.Add(b.Cid())
	}

	return out.ToSlice(), nil
}

func (api *nodeChain) Ls(ctx context.Context) <-chan interface{} {
	return api.api.node.ChainReader.BlockHistory(ctx)
}
