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

func TestSlashing(t *testing.T) {
	tf.FunctionalTest(t)

	t.Run("miner is slashed when it is late", func(t *testing.T) {
		// Give the deal time to complete
		ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fast.FilecoinOpts{
			InitOpts:   []fast.ProcessInitOption{fast.POAutoSealIntervalSeconds(1)},
			DaemonOpts: []fast.ProcessDaemonOption{fast.POBlockTime(150 * time.Millisecond)},
		})
		defer func() {
			require.NoError(t, env.Teardown(ctx))
		}()
		clientDaemon := env.GenesisMiner
		require.NoError(t, clientDaemon.MiningStart(ctx))

		// set up the daemon that will be slashing with a storage miner.
		// we need a third miner because env.GenesisMiner (clientDaemon) is a bootstrap
		// miner and therefore will never slash.
		slashingDaemon, askIDSlash := requireMinerCreateWithAsk(ctx, t, env, clientDaemon)
		defer func() {
			require.NoError(t, slashingDaemon.MiningStop(ctx))
		}()

		// minerDaemon will be slashed.
		minerDaemon, askID := requireMinerCreateWithAsk(ctx, t, env, clientDaemon)

		duration := uint64(5)
		requireMinerClientCompleteDeal(ctx, t, slashingDaemon, clientDaemon, askIDSlash, duration)
		requireMinerClientCompleteDeal(ctx, t, minerDaemon, clientDaemon, askID, duration)

		// genesis mining isn't needed any more.
		require.NoError(t, clientDaemon.MiningStop(ctx))

		// Wait until miner gets their power
		minerAddr := requireGetMinerAddress(ctx, t, minerDaemon)
		waitLimit := miner.LargestSectorSizeProvingPeriodBlocks + duration + 1
		require.NoError(t, series.WaitForBlockHeight(ctx, minerDaemon, types.NewBlockHeight(waitLimit)))
		require.NoError(t, waitForPower(ctx, t, minerDaemon, minerAddr, 1024, waitLimit))

		// miner is offline for entire proving period + grace period
		waitLimit = 1000
		require.NoError(t, minerDaemon.StopDaemon(ctx))
		assert.NoError(t, waitForPower(ctx, t, clientDaemon, minerAddr, 0, waitLimit))
	})
}

func requireMinerCreateWithAsk(ctx context.Context, t *testing.T, env *fastesting.TestEnvironment, client *fast.Filecoin) (*fast.Filecoin, uint64) {
	collateral := big.NewInt(int64(100))
	askPrice := big.NewFloat(0.5)
	expiry := big.NewInt(int64(10000))

	minerd := env.RequireNewNodeWithFunds(1234)
	require.NoError(t, series.Connect(ctx, client, minerd))
	ask, err := series.CreateStorageMinerWithAsk(ctx, minerd, collateral, askPrice, expiry)
	require.NoError(t, err)
	return minerd, ask.ID
}

func requireMinerClientCompleteDeal(ctx context.Context, t *testing.T, minerDaemon, clientDaemon *fast.Filecoin, askID uint64, duration uint64) {
	f := files.NewBytesFile([]byte("HODLHODLHODL"))
	dataCid, err := clientDaemon.ClientImport(ctx, f)
	require.NoError(t, err)

	minerAddress := requireGetMinerAddress(ctx, t, minerDaemon)

	dealResponse, err := clientDaemon.ClientProposeStorageDeal(ctx, dataCid, minerAddress, askID, duration, fast.AOAllowDuplicates(true))

	require.NoError(t, err)
	// Wait until deal is accepted and complete
	require.NoError(t, series.WaitForDealState(ctx, clientDaemon, dealResponse, storagedeal.Complete))
	bh, err := series.GetHeadBlockHeight(ctx, minerDaemon)
	require.NoError(t, err)
	fmt.Printf("Deal complete at block height %5d \n", bh.AsBigInt().Uint64())
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
		powers, err := d.MinerPower(ctx, miner)
		require.NoError(t, err)
		if expPower == powers.Power.Uint64() {
			fmt.Printf("Power reached %5d at iteration %5d \n", expPower, i)
			return nil
		}
		if i%100 == 0 {
			fmt.Printf("Power is %5d at iteration %5d \n", powers.Power.Uint64(), i)
		}
		series.CtxSleepDelay(ctx)
	}
	return errors.New(fmt.Sprintf("Miner %s power never reached %5d in %5d iterations", miner.String(), expPower, limit))
}
