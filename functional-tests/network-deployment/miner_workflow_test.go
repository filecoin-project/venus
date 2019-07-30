package networkdeployment_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/ipfs/go-ipfs-files"
	logging "github.com/ipfs/go-log"

	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/environment"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
	localplugin "github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin/local"
)

func init() {
	logging.SetDebugLogging()
}

func TestMiner(t *testing.T) {
	network := tf.DeploymentTest(t)

	ctx := context.Background()

	// Create a directory for the test using the test name (mostly for FAST)
	dir, err := ioutil.TempDir("", t.Name())
	require.NoError(t, err)

	// Create an environment to connect to the devnet
	env, err := environment.NewDevnet(network, dir)
	require.NoError(t, err)

	// Teardown will shutdown all running processes the environment knows about
	// and cleanup anything the environment setup. This includes the directory
	// the environment was created to use.
	defer func() {
		require.NoError(t, env.Teardown(ctx))
	}()

	// Setup options for nodes.
	options := make(map[string]string)
	options[localplugin.AttrLogJSON] = "0"                               // Disable JSON logs
	options[localplugin.AttrLogLevel] = "4"                              // Set log level to Info
	options[localplugin.AttrFilecoinBinary] = th.MustGetFilecoinBinary() // Set binary

	ctx = series.SetCtxSleepDelay(ctx, time.Second*30)

	genesisURI := env.GenesisCar()

	fastenvOpts := fast.FilecoinOpts{
		InitOpts:   []fast.ProcessInitOption{fast.POGenesisFile(genesisURI), fast.PODevnet(network)},
		DaemonOpts: []fast.ProcessDaemonOption{},
	}

	//
	//
	//

	miner, err := env.NewProcess(ctx, localplugin.PluginName, options, fastenvOpts)
	require.NoError(t, err)

	client, err := env.NewProcess(ctx, localplugin.PluginName, options, fastenvOpts)
	require.NoError(t, err)

	// Start Miner
	err = series.InitAndStart(ctx, miner)
	require.NoError(t, err)

	// Start Client
	err = series.InitAndStart(ctx, client)
	require.NoError(t, err)

	// Everyone needs FIL to deal with gas costs and make sure their wallets
	// exists (sending FIL to a wallet addr creates it)
	err = env.GetFunds(ctx, miner)
	require.NoError(t, err)

	err = env.GetFunds(ctx, client)
	require.NoError(t, err)

	t.Run("Verify mining", func(t *testing.T) {
		collateral := big.NewInt(10)
		price := big.NewFloat(0.000000001)
		expiry := big.NewInt(128)

		defer client.DumpLastOutput(os.Stdout)
		defer miner.DumpLastOutput(os.Stdout)

		// Create a miner on the miner node
		ask, err := series.CreateStorageMinerWithAsk(ctx, miner, collateral, price, expiry)
		require.NoError(t, err)

		// Connect the client and the miner
		err = series.Connect(ctx, client, miner)
		require.NoError(t, err)

		// Store some data with the miner with the given ask, returns the cid for
		// the imported data, and the deal which was created
		var data bytes.Buffer
		dataReader := io.LimitReader(rand.Reader, 4096)
		dataReader = io.TeeReader(dataReader, &data)
		_, deal, err := series.ImportAndStoreWithDuration(ctx, client, ask, 32, files.NewReaderFile(dataReader))
		require.NoError(t, err)

		vouchers, err := client.ClientPayments(ctx, deal.ProposalCid)
		require.NoError(t, err)

		lastVoucher := vouchers[len(vouchers)-1]

		// Wait for the deal to be complete
		err = series.WaitForDealState(ctx, client, deal, storagedeal.Complete)
		require.NoError(t, err)

		// Redeem
		err = series.WaitForBlockHeight(ctx, miner, &lastVoucher.ValidAt)
		require.NoError(t, err)

		_, err = miner.DealsRedeem(ctx, deal.ProposalCid)
		require.NoError(t, err)

		// Check PoSt / Miner Power
		// Redeem
	})
}
