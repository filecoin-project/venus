package impl

import (
	"context"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/node"
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
	ts := nd.ChainReader.Head()

	blockTime, mineDelay := nd.MiningTimes()

	getStateByKey := func(ctx context.Context, tsKey string) (state.Tree, error) {
		tsas, err := nd.ChainReader.GetTipSetAndState(ctx, tsKey)
		if err != nil {
			return nil, err
		}
		return state.LoadStateTree(ctx, nd.CborStore(), tsas.TipSetStateRoot, builtin.Actors)
	}
	getState := func(ctx context.Context, ts types.TipSet) (state.Tree, error) {
		return getStateByKey(ctx, ts.String())
	}
	getWeight := func(ctx context.Context, ts types.TipSet) (uint64, error) {
		parent, err := ts.Parents()
		if err != nil {
			return uint64(0), err
		}
		// TODO handle genesis cid more gracefully
		if parent.Len() == 0 {
			return nd.Consensus.Weight(ctx, ts, nil)
		}
		pSt, err := getStateByKey(ctx, parent.String())
		if err != nil {
			return uint64(0), err
		}
		return nd.Consensus.Weight(ctx, ts, pSt)
	}

	miningAddrIf, err := nd.PorcelainAPI.ConfigGet("mining.minerAddress")
	if err != nil {
		return nil, err
	}
	miningAddr := miningAddrIf.(address.Address)

	blockSignerAddrIf, err := nd.PorcelainAPI.ConfigGet("mining.blockSignerAddress")
	if err != nil {
		return nil, err
	}
	blockSignerAddr := blockSignerAddrIf.(address.Address)

	getAncestors := func(ctx context.Context, ts types.TipSet, newBlockHeight *types.BlockHeight) ([]types.TipSet, error) {
		return chain.GetRecentAncestors(ctx, ts, nd.ChainReader, newBlockHeight, consensus.AncestorRoundsNeeded, consensus.LookBackParameter)
	}
	worker := mining.NewDefaultWorker(nd.MsgPool, getState, getWeight, getAncestors, consensus.NewDefaultProcessor(),
		nd.PowerTable, nd.Blockstore, nd.CborStore(), miningAddr, blockSignerAddr, nd.Wallet, blockTime)

	res, err := mining.MineOnce(ctx, worker, mineDelay, ts)
	if err != nil {
		return nil, err
	}
	if res.Err != nil {
		return nil, res.Err
	}

	if err := nd.AddNewBlock(ctx, res.NewBlock); err != nil {
		return nil, err
	}

	return res.NewBlock, nil
}

func (api *nodeMining) Start(ctx context.Context) error {
	return node.StartMining(ctx, api.api.node)
}

func (api *nodeMining) Stop(ctx context.Context) error {
	api.api.node.StopMining(ctx)
	return nil
}
