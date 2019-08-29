package commands_test

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/fixtures"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/fastesting"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
)

func parseInt(t *testing.T, s string) *big.Int {
	i := new(big.Int)
	i, err := i.SetString(strings.TrimSpace(s), 10)
	assert.True(t, err, "couldn't parse as big.Int %q", s)
	return i
}

func TestMiningGenBlock(t *testing.T) {
	tf.IntegrationTest(t)

	d := makeTestDaemonWithMinerAndStart(t)
	defer d.ShutdownSuccess()

	addr := fixtures.TestAddresses[0]

	s := d.RunSuccess("wallet", "balance", addr)
	beforeBalance := parseInt(t, s.ReadStdout())

	d.RunSuccess("mining", "once")

	s = d.RunSuccess("wallet", "balance", addr)
	afterBalance := parseInt(t, s.ReadStdout())
	sum := new(big.Int)

	assert.Equal(t, sum.Add(beforeBalance, big.NewInt(1000)), afterBalance)
}

func TestMiningSealNow(t *testing.T) {
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

	// Calls MiningOnce on genesis (client). This also starts the Miner.
	pparams, err := minerNode.Protocol(ctx)
	require.NoError(t, err)

	sinfo := pparams.SupportedSectors[0]

	// mine the create storage message, then mine the set ask message
	series.CtxMiningNext(ctx, 2)

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

	// Since the miner does not yet have power, we still need the genesis node to mine
	// the miner's commitSector and the submitPoSt messages
	series.CtxMiningNext(ctx, 2)

	// start sealing
	err = minerNode.SealNow(ctx)
	require.NoError(t, err)

	// We know the miner has sealed and committed a sector if their power increases on chain.
	// Wait up to 3 minutes for that to happen.
	for i := 0; i < 180; i++ {
		power, err := minerNode.MinerPower(ctx, miningAddress)
		require.NoError(t, err)

		if power.Power.GreaterThan(types.ZeroBytes) {
			// miner has gained power, so seal was successful
			return
		}
		time.Sleep(time.Second)
	}
	assert.Fail(t, "timed out waiting for miner to gain power from sealing")
}
