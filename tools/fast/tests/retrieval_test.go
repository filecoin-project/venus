package tests

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"io/ioutil"
	"math/big"
	"testing"
	"time"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
	files "github.com/ipfs/go-ipfs-files"
	logging "github.com/ipfs/go-log/v2"

	"github.com/stretchr/testify/require"

	th "github.com/filecoin-project/venus/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/tools/fast"
	"github.com/filecoin-project/venus/tools/fast/environment"
	"github.com/filecoin-project/venus/tools/fast/series"
	localplugin "github.com/filecoin-project/venus/tools/iptb-plugins/filecoin/local"
)

func init() {
	// Enabling debug logging provides a lot of insight into what commands are
	// being executed
	logging.SetDebugLogging()
}

// TestRetrieval exercises storing and retrieving with the filecoin protocols using a locally running
// temporary network.
func TestRetrievalLocalNetwork(t *testing.T) {
	tf.FunctionalTest(t)
	t.Skip("Long term solution: #3642")

	blocktime := time.Second * 5

	// This test should run in 20 block times, with 120 seconds for sealing, and no longer
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(20*blocktime).Add(120*time.Second))
	defer cancel()

	// Create a directory for the test using the test name (mostly for FAST)
	dir, err := ioutil.TempDir("", t.Name())
	require.NoError(t, err)

	// Create an environment that includes a genesis block with 1MM FIL
	env, err := environment.NewMemoryGenesis(big.NewInt(1000000), dir)
	require.NoError(t, err)

	// Teardown will shutdown all running processes the environment knows about
	// and cleanup anything the evironment setup. This includes the directory
	// the environment was created to use.
	defer func() {
		require.NoError(t, env.Teardown(ctx))
	}()

	// Setup options for nodes.
	options := make(map[string]string)
	options[localplugin.AttrLogJSON] = "0"                               // Disable JSON logs
	options[localplugin.AttrLogLevel] = "4"                              // Set log level to Info
	options[localplugin.AttrFilecoinBinary] = th.MustGetFilecoinBinary() // Set binary

	ctx = series.SetCtxSleepDelay(ctx, blocktime)

	genesisURI := env.GenesisCar()
	genesisMiner, err := env.GenesisMiner()
	require.NoError(t, err)

	fastenvOpts := fast.FilecoinOpts{
		InitOpts:   []fast.ProcessInitOption{fast.POGenesisFile(genesisURI)},
		DaemonOpts: []fast.ProcessDaemonOption{fast.POBlockTime(blocktime)},
	}

	// Setup nodes used for the test
	genesis, err := env.NewProcess(ctx, localplugin.PluginName, options, fastenvOpts)
	require.NoError(t, err)

	miner, err := env.NewProcess(ctx, localplugin.PluginName, options, fastenvOpts)
	require.NoError(t, err)

	client, err := env.NewProcess(ctx, localplugin.PluginName, options, fastenvOpts)
	require.NoError(t, err)

	// Start setting up the nodes
	// Setup Genesis
	err = series.SetupGenesisNode(ctx, genesis, genesisMiner.Address, files.NewReaderFile(genesisMiner.Owner))
	require.NoError(t, err)

	err = genesis.MiningStart(ctx)
	require.NoError(t, err)

	// Start Miner
	err = series.InitAndStart(ctx, miner)
	require.NoError(t, err)

	// Start Client
	err = series.InitAndStart(ctx, client)
	require.NoError(t, err)

	// Connect everything to the genesis node so it can issue filecoin when needed
	err = series.Connect(ctx, genesis, miner)
	require.NoError(t, err)

	err = series.Connect(ctx, genesis, client)
	require.NoError(t, err)

	// Everyone needs FIL to deal with gas costs and make sure their wallets
	// exists (sending FIL to a wallet addr creates it)
	err = series.SendFilecoinDefaults(ctx, genesis, miner, 1000)
	require.NoError(t, err)

	err = series.SendFilecoinDefaults(ctx, genesis, client, 1000)
	require.NoError(t, err)

	RunRetrievalTest(ctx, t, miner, client)
}

// TestRetrieval exercises storing and retreiving with the filecoin protocols on a kittyhawk deployed
// devnet.
func TestRetrievalDevnet(t *testing.T) {
	tf.FunctionalTest(t)

	// Skip the test so it doesn't run
	t.SkipNow()

	blocktime := time.Second * 30
	networkConfig, err := environment.FindDevnetConfigByName("nightly")
	require.NoError(t, err)

	// This test should run in and hour and no longer
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(blocktime*120))
	defer cancel()

	// Create a directory for the test using the test name (mostly for FAST)
	dir, err := ioutil.TempDir("", t.Name())
	require.NoError(t, err)

	// Create an environment that includes a genesis block with 1MM FIL
	env, err := environment.NewDevnet(networkConfig, dir)
	require.NoError(t, err)

	// Teardown will shutdown all running processes the environment knows about
	// and cleanup anything the evironment setup. This includes the directory
	// the environment was created to use.
	defer func() {
		require.NoError(t, env.Teardown(ctx))
	}()

	// Setup options for nodes.
	options := make(map[string]string)
	options[localplugin.AttrLogJSON] = "0"                               // Disable JSON logs
	options[localplugin.AttrLogLevel] = "4"                              // Set log level to Info
	options[localplugin.AttrFilecoinBinary] = th.MustGetFilecoinBinary() // Set binary

	ctx = series.SetCtxSleepDelay(ctx, blocktime)

	genesisURI := env.GenesisCar()

	fastenvOpts := fast.FilecoinOpts{
		InitOpts:   []fast.ProcessInitOption{fast.POGenesisFile(genesisURI), fast.PODevnet(networkConfig.Name)},
		DaemonOpts: []fast.ProcessDaemonOption{},
	}

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

	RunRetrievalTest(ctx, t, miner, client)
}

func RunRetrievalTest(ctx context.Context, t *testing.T, miner, client *fast.Filecoin) {
	collateral := big.NewInt(10)            // FIL
	price := big.NewFloat(0.000000001)      // price per byte/block
	expiry := big.NewInt(24 * 60 * 60 / 30) // ~24 hours

	pparams, err := miner.Protocol(ctx)
	require.NoError(t, err)

	sinfo := pparams.SupportedSectors[0]

	// Create a miner on the miner node
	ask, err := series.CreateStorageMinerWithAsk(ctx, miner, collateral, price, expiry, sinfo.Size)
	require.NoError(t, err)

	// Connect the client and the miner
	require.NoError(t, series.Connect(ctx, client, miner))

	// Start the miner
	require.NoError(t, miner.MiningStart(ctx))

	// Store some data with the miner with the given ask, returns the cid for
	// the imported data, and the deal which was created
	var data bytes.Buffer
	dataReader := io.LimitReader(rand.Reader, int64(sinfo.MaxPieceSize))
	dataReader = io.TeeReader(dataReader, &data)
	dcid, deal, err := series.ImportAndStore(ctx, client, ask, files.NewReaderFile(dataReader))
	require.NoError(t, err)

	// Wait for the deal to be complete
	proposalResponse, err := series.WaitForDealState(ctx, client, deal, storagemarket.StorageDealActive)
	require.NoError(t, err)

	_, err = client.MessageWait(ctx, *proposalResponse.PublishMessage)
	require.NoError(t, err)

	// Verify PIP
	_, err = client.ClientVerifyStorageDeal(ctx, deal.Proposal)
	require.NoError(t, err)

	// Retrieve the stored piece of data
	reader, err := client.RetrievalClientRetrievePiece(ctx, dcid, ask.Miner)
	require.NoError(t, err)

	// Verify that it's all the same
	retrievedData, err := ioutil.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, data.Bytes(), retrievedData)
}
