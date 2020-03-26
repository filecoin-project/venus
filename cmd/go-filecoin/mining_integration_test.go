package commands_test

import (
	"context"
	"math/big"
	"testing"
	"time"

	fbig "github.com/filecoin-project/specs-actors/actors/abi/big"
	specsbig "github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/fixtures"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/fastesting"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
)

func TestMiningGenBlock(t *testing.T) {
	tf.IntegrationTest(t)
	t.Skip("Dragons: fake proofs")
	ctx := context.Background()
	builder := test.NewNodeBuilder(t)
	buildWithMiner(t, builder)

	n := builder.BuildAndStart(ctx)
	defer n.Stop(ctx)

	addr := fixtures.TestAddresses[0]

	attoFILBefore, err := n.PorcelainAPI.WalletBalance(ctx, addr)
	require.NoError(t, err)

	_, err = n.BlockMining.BlockMiningAPI.MiningOnce(ctx)
	require.NoError(t, err)

	attoFILAfter, err := n.PorcelainAPI.WalletBalance(ctx, addr)
	require.NoError(t, err)

	assert.Equal(t, specsbig.Add(attoFILBefore, types.NewAttoTokenFromToken(1000)), attoFILAfter)
}

func TestMiningAddPieceAndSealNow(t *testing.T) {
	t.Skip("Long term solution: #3642")
	tf.FunctionalTest(t)

	ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fast.FilecoinOpts{
		InitOpts:   []fast.ProcessInitOption{fast.POAutoSealIntervalSeconds(1)},
		DaemonOpts: []fast.ProcessDaemonOption{fast.POBlockTime(50 * time.Millisecond)},
	})
	defer func() {
		require.NoError(t, env.Teardown(ctx))
	}()

	genesisNode := env.GenesisMiner

	minerNode := env.RequireNewNodeWithFunds(1000)

	// Connect the clientNode and the minerNode
	require.NoError(t, series.Connect(ctx, genesisNode, minerNode))

	pparams, err := minerNode.Protocol(ctx)
	require.NoError(t, err)

	sinfo := pparams.SupportedSectors[0]

	// start mining so we get to a block height that
	require.NoError(t, genesisNode.MiningStart(ctx))
	defer func() {
		require.NoError(t, genesisNode.MiningStop(ctx))
	}()

	_, err = series.CreateStorageMinerWithAsk(ctx, minerNode, big.NewInt(500), big.NewFloat(0.0001), big.NewInt(3000), sinfo.Size)
	require.NoError(t, err)

	// get address of miner so we can check power
	miningAddress, err := minerNode.MiningAddress(ctx)
	require.NoError(t, err)

	// start mining for miner node to seal and schedule PoSting
	require.NoError(t, minerNode.MiningStart(ctx))
	defer func() {
		require.NoError(t, minerNode.MiningStop(ctx))
	}()

	// add a piece
	//_, err = minerNode.AddPiece(ctx, files.NewBytesFile([]byte("HODL")))
	//require.NoError(t, err)

	// start sealing
	err = minerNode.SealNow(ctx)
	require.NoError(t, err)

	// We know the miner has sealed and committed a sector if their power increases on chain.
	// Wait up to 300 seconds for that to happen.
	for i := 0; i < 300; i++ {
		power, err := minerNode.MinerStatus(ctx, miningAddress)
		require.NoError(t, err)

		if power.Power.GreaterThan(fbig.Zero()) {
			// miner has gained power, so seal was successful
			return
		}
		time.Sleep(time.Second)
	}
	assert.Fail(t, "timed out waiting for miner to gain power from sealing")
}
