package impl

import (
	"context"

	"gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

type NodeMining struct {
	api *NodeAPI
}

func NewNodeMining(api *NodeAPI) *NodeMining {
	return &NodeMining{api: api}
}

func (api *NodeMining) Once(ctx context.Context) (*types.Block, error) {
	nd := api.api.node
	ts := nd.ChainMgr.GetHeaviestTipSet()

	if nd.RewardAddress().Empty() {
		return nil, ErrMissingRewardAddress
	}
	blockGenerator := mining.NewBlockGenerator(nd.MsgPool, func(ctx context.Context, ts core.TipSet) (state.Tree, datastore.Datastore, error) {
		return nd.ChainMgr.State(ctx, ts.ToSlice())
	}, nd.ChainMgr.Weight, core.ApplyMessages, nd.ChainMgr.PwrTableView)

	miningAddr, err := nd.MiningAddress()
	if err != nil {
		return nil, err
	}

	res := mining.MineOnce(ctx, mining.NewWorker(blockGenerator), ts, nd.RewardAddress(), miningAddr)
	if res.Err != nil {
		return nil, res.Err
	}

	if err := nd.AddNewBlock(ctx, res.NewBlock); err != nil {
		return nil, err
	}

	return res.NewBlock, nil
}

func (api *NodeMining) Start() error {
	return api.api.node.StartMining()
}

func (api *NodeMining) Stop() error {
	api.api.node.StopMining()
	return nil
}
