package protocolapi_test

import (
	"context"
	"github.com/filecoin-project/go-filecoin/protocol/storage"
	"github.com/filecoin-project/go-filecoin/protocolapi"
	ast "gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	req "gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
	"testing"

	"github.com/filecoin-project/go-filecoin/node"
)

func TestNew(t *testing.T) {
	assert := ast.New(t)
	require := req.New(t)

	api, _ := newAPI(t, assert)
	require.NotNil(t, api)

	assert.NotNil(api.Syncer)
	assert.NotNil(api.TicketSigner)
	assert.NotNil(api.OnlineStore)
}

func TestAPI_MineOnce(t *testing.T) {
	assert := ast.New(t)
	require := req.New(t)
	ctx := context.Background()

	api, nd := newAPI(t, assert)
	require.NoError(nd.Start(ctx))
	defer nd.Stop(ctx)

	blk, err := api.MineOnce(ctx)
	require.Nil(err)
	require.NotNil(blk)
	assert.NotNil(blk.Ticket)
}

func newAPI(t *testing.T, assert *ast.Assertions) (protocolapi.API, *node.Node) {
	seed := node.MakeChainSeed(t, node.TestGenCfg)
	configOpts := []node.ConfigOpt{}

	nd := node.MakeNodeWithChainSeed(t, seed, configOpts,
		node.AutoSealIntervalSecondsOpt(1),
	)
	bt, md := nd.MiningTimes()
	seed.GiveKey(t, nd, 0)
	mineraddr, minerOwnerAddr := seed.GiveMiner(t, nd, 0)
	_, err := storage.NewMiner(mineraddr, minerOwnerAddr, nd, nd.Repo.DealsDatastore(), nd.PorcelainAPI)
	assert.NoError(err)
	return protocolapi.New(
		nd.AddNewBlock,
		nd.Blockstore,
		nd.CborStore(), nd.OnlineStore,
		nd.ChainReader,
		nd.Consensus,
		bt, md,
		nd.MsgPool,
		nd.PorcelainAPI,
		nd.PowerTable,
		nd.Syncer,
		nd.Wallet), nd
}
