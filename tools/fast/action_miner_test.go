package fast_test

import (
	"context"
	"math/big"
	"testing"
	"time"

	files "github.com/ipfs/go-ipfs-files"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/fastesting"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
)

func TestFilecoin_MinerPower(t *testing.T) {
	tf.IntegrationTest(t)

	// Give the deal time to complete
	ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fast.FilecoinOpts{
		InitOpts:   []fast.ProcessInitOption{fast.POAutoSealIntervalSeconds(1)},
		DaemonOpts: []fast.ProcessDaemonOption{fast.POBlockTime(100 * time.Millisecond)},
	})

	defer func() {
		require.NoError(t, env.Teardown(ctx))
	}()

	clientDaemon := env.GenesisMiner
	require.NoError(t, clientDaemon.MiningStart(ctx))
	defer func() {
		require.NoError(t, clientDaemon.MiningStop(ctx))
	}()

	minerDaemon := env.RequireNewNodeWithFunds(10000)
	_, _ = requireMinerClientMakeADeal(ctx, t, minerDaemon, clientDaemon)

	assertPowerOutput(ctx, t, minerDaemon, 0, 1024)
}

func requireMinerClientMakeADeal(ctx context.Context, t *testing.T, minerDaemon, clientDaemon *fast.Filecoin) (*storagedeal.Response, uint64) {
	collateral := big.NewInt(int64(1))
	price := big.NewFloat(float64(1))
	expiry := big.NewInt(int64(10000))
	_, err := series.CreateStorageMinerWithAsk(ctx, minerDaemon, collateral, price, expiry)
	require.NoError(t, err)

	f := files.NewBytesFile([]byte("HODLHODLHODL"))
	dataCid, err := clientDaemon.ClientImport(ctx, f)
	require.NoError(t, err)

	minerAddress := requireGetMinerAddress(ctx, t, minerDaemon)

	dealDuration := uint64(5)
	dealResponse, err := clientDaemon.ClientProposeStorageDeal(ctx, dataCid, minerAddress, 0, dealDuration)
	require.NoError(t, err)
	return dealResponse, dealDuration
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
	assert.Equal(t, actualMinerPwr.Power.Uint64(), expMinerPwr, "for miner power")
	assert.Equal(t, actualMinerPwr.Total.Uint64(), expTotalPwr, "for total power")
}
