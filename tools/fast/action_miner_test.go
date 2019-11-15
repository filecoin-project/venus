package fast_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
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

	// TODO 3642 this test should check that miner's created with miner create have power
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
