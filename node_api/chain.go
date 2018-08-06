package node_api

import (
	"context"

	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
)

type ChainAPI struct {
	api *API
}

func NewChainAPI(api *API) *ChainAPI {
	return &ChainAPI{api: api}
}

func (api *ChainAPI) Head() ([]*cid.Cid, error) {
	ts := api.api.node.ChainMgr.GetHeaviestTipSet()
	if len(ts) == 0 {
		return nil, ErrHeaviestTipSetNotFound
	}
	ts_slice := ts.ToSlice()
	out := make([]*cid.Cid, len(ts_slice))
	for i, b := range ts_slice {
		out[i] = b.Cid()
	}

	return out, nil
}

func (api *ChainAPI) Ls(ctx context.Context) <-chan interface{} {
	return api.api.node.ChainMgr.BlockHistory(ctx)
}
