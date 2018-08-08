package node_api

import (
	"context"

	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
)

type NodeChain struct {
	api *API
}

func NewNodeChain(api *API) *NodeChain {
	return &NodeChain{api: api}
}

func (api *NodeChain) Head() ([]*cid.Cid, error) {
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

func (api *NodeChain) Ls(ctx context.Context) <-chan interface{} {
	return api.api.node.ChainMgr.BlockHistory(ctx)
}
