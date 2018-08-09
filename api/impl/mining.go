package impl

import (
	"context"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

type nodeMining struct {
	api *nodeAPI
}

func newNodeMining(api *nodeAPI) *nodeMining {
	return &nodeMining{api: api}
}

func (api *nodeMining) Once(ctx context.Context) (*types.Block, error) {
	nd := api.api.node
	ts := nd.ChainMgr.GetHeaviestTipSet()

	miningAddr, err := nd.MiningAddress()
	if err != nil {
		return nil, err
	}

	worker := mining.NewMiningWorker(nd.MsgPool, func(ctx context.Context, ts core.TipSet) (state.Tree, error) {
		return nd.ChainMgr.State(ctx, ts.ToSlice())
	}, nd.ChainMgr.Weight, core.ApplyMessages, nd.ChainMgr.PwrTableView, nd.Blockstore, nd.CborStore, miningAddr)

	res := mining.MineOnce(ctx, mining.NewScheduler(worker), ts)
	if res.Err != nil {
		return nil, res.Err
	}

	if err := nd.AddNewBlock(ctx, res.NewBlock); err != nil {
		return nil, err
	}

	return res.NewBlock, nil
}

func (api *nodeMining) Start() error {
	return api.api.node.StartMining()
}

func (api *nodeMining) Stop() error {
	api.api.node.StopMining()
	return nil
}
