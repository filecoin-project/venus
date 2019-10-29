package fast_test

import (
	"context"
	"math/big"
	"testing"

	"github.com/filecoin-project/go-filecoin/tools/fast/series"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/fastesting"
)

func TestFilecoin_MinerPower(t *testing.T) {
	tf.IntegrationTest(t)

	ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fast.FilecoinOpts{})
	defer func() {
		require.NoError(t, env.Teardown(ctx))
	}()

	expectedGenesisPower := uint64(131072)
	assertPowerOutput(ctx, t, env.GenesisMiner, expectedGenesisPower, expectedGenesisPower)

	minerDaemon := env.RequireNewNodeWithFunds(10000)
	requireMiner(ctx, t, minerDaemon, env.GenesisMiner)

	assertPowerOutput(ctx, t, minerDaemon, 0, expectedGenesisPower)
}

func requireMiner(ctx context.Context, t *testing.T, minerDaemon, clientDaemon *fast.Filecoin) {
	collateral := big.NewInt(int64(1))

	pparams, err := minerDaemon.Protocol(ctx)
	require.NoError(t, err)

	sinfo := pparams.SupportedSectors[0]

	// mine the create storage message
	series.CtxMiningNext(ctx, 1)

	// Create miner
	_, err = minerDaemon.MinerCreate(ctx, collateral, fast.AOSectorSize(sinfo.Size), fast.AOPrice(big.NewFloat(1.0)), fast.AOLimit(300))
	require.NoError(t, err)
}

func requireGetMinerAddress(ctx context.Context, t *testing.T, daemon *fast.Filecoin) address.Address {
	var minerAddress address.Address
	err := daemon.ConfigGet(ctx, "mining.minerAddress", &minerAddress)
	require.NoError(t, err)
	return minerAddress
}

func assertPowerOutput(ctx context.Context, t *testing.T, d *fast.Filecoin, expMinerPwr, expTotalPwr uint64) {
	minerAddr := requireGetMinerAddress(ctx, t, d)
	actualMinerPwr, err := d.MinerPower(ctx, minerAddr)
	require.NoError(t, err)
	assert.Equal(t, expMinerPwr, actualMinerPwr.Power.Uint64(), "for miner power")
	assert.Equal(t, expTotalPwr, actualMinerPwr.Total.Uint64(), "for total power")
}
