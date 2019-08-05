package tests

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	files "github.com/ipfs/go-ipfs-files"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/fastesting"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
	"github.com/filecoin-project/go-filecoin/types"
)

// Not passing, and also too slow to let run in CI
func TestSlashing(t *testing.T) {
	tf.IntegrationTest(t)
	t.SkipNow()

	t.Run("miner is slashed when it is late", func(t *testing.T) {
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

		minerDaemon := env.RequireNewNodeWithFunds(1111)
		require.NoError(t, series.Connect(ctx, clientDaemon, minerDaemon))

		duration := uint64(5)
		askID := requireMinerCreateWithAsk(ctx, t, minerDaemon)
		minerAddr := requireGetMinerAddress(ctx, t, minerDaemon)
		dealResponse := requireMinerClientMakeADeal(ctx, t, minerDaemon, clientDaemon, askID, duration)

		// Wait until deal is accepted and complete
		require.NoError(t, series.WaitForDealState(ctx, clientDaemon, dealResponse, storagedeal.Complete))

		// Wait until miner gets their power
		waitLimit := miner.LargestSectorSizeProvingPeriodBlocks + duration + 1
		require.NoError(t, series.WaitForBlockHeight(ctx, minerDaemon, types.NewBlockHeight(waitLimit)))
		require.NoError(t, waitForPower(ctx, t, minerDaemon, minerAddr, 1024, waitLimit))

		// miner is offline for entire proving period + grace period
		require.NoError(t, minerDaemon.StopDaemon(ctx))

		waitLimit = 1000
		assert.NoError(t, waitForPower(ctx, t, clientDaemon, minerAddr, 0, waitLimit))
	})

	// start genesis node mining
	// set up another miner with commits
	// verify normal operation of storage fault monitor when there is a new tipset
	//  (it doesn't crash?)

	// 0. make miner submit proof on time
	//    verify normal operation of storage fault monitor
	//    verify miner is not slashed

	// 1. make miner be late by not submitting proof
	//    verify the miner is slashed

	// 2. make miner be late but submits late proof
	//    verify miner is not slashed twice

}

func requireMinerCreateWithAsk(ctx context.Context, t *testing.T, d *fast.Filecoin) uint64 {
	collateral := big.NewInt(int64(100))
	askPrice := big.NewFloat(0.5)
	expiry := big.NewInt(int64(10000))
	ask, err := series.CreateStorageMinerWithAsk(ctx, d, collateral, askPrice, expiry)
	require.NoError(t, err)
	return ask.ID
}

func requireMinerClientMakeADeal(ctx context.Context, t *testing.T, minerDaemon, clientDaemon *fast.Filecoin, askID uint64, duration uint64) *storagedeal.Response {
	f := files.NewBytesFile([]byte("HODLHODLHODL"))
	dataCid, err := clientDaemon.ClientImport(ctx, f)
	require.NoError(t, err)

	minerAddress := requireGetMinerAddress(ctx, t, minerDaemon)

	dealResponse, err := clientDaemon.ClientProposeStorageDeal(ctx, dataCid, minerAddress, askID, duration, fast.AOAllowDuplicates(true))

	require.NoError(t, err)
	return dealResponse
}

func requireGetMinerAddress(ctx context.Context, t *testing.T, daemon *fast.Filecoin) address.Address {
	var minerAddress address.Address
	err := daemon.ConfigGet(ctx, "mining.minerAddress", &minerAddress)
	require.NoError(t, err)
	return minerAddress
}

// waitForPower queries miner power for up to limit iterations, until it has power expPower.
func waitForPower(ctx context.Context, t *testing.T, d *fast.Filecoin, miner address.Address, expPower, limit uint64) error {
	for i := uint64(0); i < limit; i++ {
		actualPower, _, err := d.MinerPower(ctx, miner)
		require.NoError(t, err)
		if expPower == actualPower.Uint64() {
			return nil
		}
		series.CtxSleepDelay(ctx)
	}
	return errors.New(fmt.Sprintf("Miner %s power never reached %5d in %5d iterations", miner.String(), expPower, limit))
}
