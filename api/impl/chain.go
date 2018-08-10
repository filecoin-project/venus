package impl

import (
	"context"

	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
)

type nodeChain struct {
	api *nodeAPI
}

func newNodeChain(api *nodeAPI) *nodeChain {
	return &nodeChain{api: api}
}

func (api *nodeChain) Head() ([]*cid.Cid, error) {
	ts := api.api.node.ChainMgr.GetHeaviestTipSet()
	if len(ts) == 0 {
		return nil, ErrHeaviestTipSetNotFound
	}
	tsSlice := ts.ToSlice()
	out := make([]*cid.Cid, len(tsSlice))
	for i, b := range tsSlice {
		out[i] = b.Cid()
	}

	return out, nil
}

func (api *nodeChain) Ls(ctx context.Context) <-chan interface{} {
	return api.api.node.ChainMgr.BlockHistory(ctx)
}
