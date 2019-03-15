package block_test

import (
	"context"
	bapi "github.com/filecoin-project/go-filecoin/protocol/block"
	"github.com/filecoin-project/go-filecoin/protocol/storage"
	ast "gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	req "gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
	"testing"

	"github.com/filecoin-project/go-filecoin/node"
)

func TestTrivialNew(t *testing.T) {
	assert := ast.New(t)
	require := req.New(t)

	api, _ := newAPI(t, assert)
	require.NotNil(t, api)
}

func TestAPI_MineOnce(t *testing.T) {
	assert := ast.New(t)
	require := req.New(t)
	ctx := context.Background()

	api, nd := newAPI(t, assert)
	require.NoError(nd.Start(ctx))
	defer nd.Stop(ctx)

	blk, err := api.MiningOnce(ctx)
	require.Nil(err)
	require.NotNil(blk)
	assert.NotNil(blk.Ticket)
}

func TestAPI_StartStopMining(t *testing.T) {
	assert := ast.New(t)
	require := req.New(t)
	ctx := context.Background()

	api, nd := newAPI(t, assert)
	require.NoError(nd.Start(ctx))
	defer nd.Stop(ctx)

	require.NoError(api.MiningStart(ctx))
	api.MiningStop(ctx)
}

func newAPI(t *testing.T, assert *ast.Assertions) (bapi.API, *node.Node) {
	seed := node.MakeChainSeed(t, node.TestGenCfg)
	configOpts := []node.ConfigOpt{}

	nd := node.MakeNodeWithChainSeed(t, seed, configOpts,
		node.AutoSealIntervalSecondsOpt(1),
	)
	bt, md := nd.MiningTimes()
	seed.GiveKey(t, nd, 0)
	mAddr, moAddr := seed.GiveMiner(t, nd, 0)
	_, err := storage.NewMiner(mAddr, moAddr, nd, nd.Repo.DealsDatastore(), nd.PorcelainAPI)
	assert.NoError(err)
	return bapi.New(
		nd.AddNewBlock,
		nd.Blockstore,
		nd.CborStore(),
		nd.ChainReader,
		nd.Consensus,
		bt, md,
		nd.MsgPool,
		nd.PorcelainAPI,
		nd.PowerTable,
		nd.StartMining,
		nd.StopMining,
		nd.Syncer,
		nd.Wallet), nd
}
