package mining_test

import (
	"context"
	"testing"

	bapi "github.com/filecoin-project/go-filecoin/internal/pkg/protocol/mining"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestTrivialNew(t *testing.T) {
	tf.UnitTest(t)

	api, _ := newAPI(t)
	require.NotNil(t, api)
}

func TestAPI_MineOnce(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	api, nd := newAPI(t)
	require.NoError(t, nd.Start(ctx))
	defer nd.Stop(ctx)

	blk, err := api.MiningOnce(ctx)
	require.Nil(t, err)
	require.NotNil(t, blk)
}

func TestMiningAPI_MiningSetup(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	api, nd := newAPI(t)
	require.NoError(t, nd.Start(ctx))
	defer nd.Stop(ctx)

	require.NoError(t, api.MiningSetup(ctx))
	assert.NotNil(t, nd.PieceManager())
}

func TestMiningAPI_MiningStart(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	api, nd := newAPI(t)
	require.NoError(t, nd.Start(ctx))
	defer nd.Stop(ctx)

	require.NoError(t, api.MiningStart(ctx))
	assert.True(t, nd.IsMining())
	nd.StopMining(ctx)
}

func TestMiningAPI_MiningIsActive(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	api, nd := newAPI(t)
	require.NoError(t, nd.Start(ctx))
	defer nd.Stop(ctx)

	require.NoError(t, nd.StartMining(ctx))
	assert.True(t, api.MiningIsActive())
	nd.StopMining(ctx)
	assert.False(t, api.MiningIsActive())

	nd.StopMining(ctx)
}

func TestMiningAPI_MiningStop(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	api, nd := newAPI(t)
	require.NoError(t, nd.Start(ctx))
	defer nd.Stop(ctx)

	require.NoError(t, nd.StartMining(ctx))
	api.MiningStop(ctx)
	assert.False(t, nd.IsMining())
}

func TestMiningAPI_MiningAddress(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	api, nd := newAPI(t)

	require.NoError(t, nd.Start(ctx))
	defer nd.Stop(ctx)

	require.NoError(t, nd.StartMining(ctx))

	maybeAddress, err := api.MinerAddress()
	require.NoError(t, err)
	minerAddress, err := nd.MiningAddress()
	require.NoError(t, err)

	assert.Equal(t, minerAddress, maybeAddress)

	nd.StopMining(ctx)
}

func TestMiningAPI_MiningTogether(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	api, nd := newAPI(t)
	require.NoError(t, nd.Start(ctx))
	defer nd.Stop(ctx)

	require.NoError(t, api.MiningStart(ctx))
	assert.True(t, nd.IsMining())
	blk, err := api.MiningOnce(ctx)
	require.Nil(t, blk)
	require.Contains(t, err.Error(), "Node is already mining")
	nd.StopMining(ctx)
	blk, err = api.MiningOnce(ctx)
	require.Nil(t, err)
	require.NotNil(t, blk)
}

func newAPI(t *testing.T) (bapi.API, *node.Node) {
	seed := node.MakeChainSeed(t, node.TestGenCfg)
	ctx := context.Background()
	builder := test.NewNodeBuilder(t)
	builder.WithGenesisInit(seed.GenesisInitFunc)
	nd := builder.Build(ctx)
	seed.GiveKey(t, nd, 0)
	seed.GiveMiner(t, nd, 0) // TODO: go-fil-markets integration
	return bapi.New(
		nd.MiningAddress,
		nd.AddNewBlock,
		nd.Chain().ChainReader,
		nd.IsMining,
		nd.SetupMining,
		nd.StartMining,
		nd.StopMining,
		nd.CreateMiningWorker,
		nd.ChainClock,
	), nd
}
