package block_test

import (
	"context"
	bapi "github.com/filecoin-project/go-filecoin/protocol/block"
	"github.com/filecoin-project/go-filecoin/protocol/storage"
	"github.com/filecoin-project/go-filecoin/types"

	ast "github.com/stretchr/testify/assert"
	req "github.com/stretchr/testify/require"
	"testing"

	"github.com/filecoin-project/go-filecoin/node"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

func TestTrivialNew(t *testing.T) {
	tf.UnitTest(t)

	assert := ast.New(t)
	require := req.New(t)

	api, _ := newAPI(t, assert)
	require.NotNil(t, api)
}

func TestAPI_MineOnce(t *testing.T) {
	tf.UnitTest(t)

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

func TestMiningAPI_MiningStart(t *testing.T) {
	tf.UnitTest(t)

	assert := ast.New(t)
	require := req.New(t)
	ctx := context.Background()
	api, nd := newAPI(t, assert)
	require.NoError(nd.Start(ctx))
	defer nd.Stop(ctx)

	require.NoError(api.MiningStart(ctx))
	assert.True(nd.IsMining())
	nd.StopMining(ctx)
}

func TestMiningAPI_MiningIsActive(t *testing.T) {
	tf.UnitTest(t)

	assert := ast.New(t)
	require := req.New(t)
	ctx := context.Background()
	api, nd := newAPI(t, assert)
	require.NoError(nd.Start(ctx))
	defer nd.Stop(ctx)

	require.NoError(nd.StartMining(ctx))
	assert.True(api.MiningIsActive())
	nd.StopMining(ctx)
	assert.False(api.MiningIsActive())

	nd.StopMining(ctx)
}

func TestMiningAPI_MiningStop(t *testing.T) {
	tf.UnitTest(t)

	assert := ast.New(t)
	require := req.New(t)
	ctx := context.Background()
	api, nd := newAPI(t, assert)

	require.NoError(nd.Start(ctx))
	defer nd.Stop(ctx)

	require.NoError(nd.StartMining(ctx))
	api.MiningStop(ctx)
	assert.False(nd.IsMining())
}

func TestMiningAPI_MiningAddress(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	api, nd := newAPI(t, ast.New(t))

	ast.NoError(t, nd.Start(ctx))
	defer nd.Stop(ctx)

	req.NoError(t, nd.StartMining(ctx))

	maybeAddress, err := api.MinerAddress()
	req.NoError(t, err)
	minerAddress, err := nd.MiningAddress()
	req.NoError(t, err)

	ast.Equal(t, minerAddress, maybeAddress)

	nd.StopMining(ctx)

}

func newAPI(t *testing.T, assert *ast.Assertions) (bapi.MiningAPI, *node.Node) {
	seed := node.MakeChainSeed(t, node.TestGenCfg)
	configOpts := []node.ConfigOpt{}

	nd := node.MakeNodeWithChainSeed(t, seed, configOpts,
		node.AutoSealIntervalSecondsOpt(1),
	)
	bt := nd.PorcelainAPI.BlockTime()
	seed.GiveKey(t, nd, 0)
	mAddr, ownerAddr := seed.GiveMiner(t, nd, 0)
	_, err := storage.NewMiner(mAddr, ownerAddr, ownerAddr, &storage.FakeProver{}, types.OneKiBSectorSize, nd, nd.Repo.DealsDatastore(), nd.PorcelainAPI)
	assert.NoError(err)
	return bapi.New(
		nd.MiningAddress,
		nd.AddNewBlock,
		nd.ChainReader,
		nd.IsMining,
		bt,
		nd.StartMining,
		nd.StopMining,
		nd.CreateMiningWorker), nd
}
