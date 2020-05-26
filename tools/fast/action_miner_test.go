package fast_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/fastesting"
)

func TestFilecoin_MinerPower(t *testing.T) {
	tf.IntegrationTest(t)
	t.Skip("not working")

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
	status, err := d.MinerStatus(ctx, minerAddr)
	require.NoError(t, err)
	assert.Equal(t, expMinerPwr, status.QualityAdjustedPower.Uint64(), "for miner power")
	assert.Equal(t, expTotalPwr, status.NetworkQualityAdjustedPower.Uint64(), "for total power")
}
