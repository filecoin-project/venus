package tests

import (
	"bytes"
	"context"
	"io/ioutil"
	"math/big"
	"testing"
	"time"

	"gx/ipfs/QmQmhotPUzVrMEWNK3x1R5jQ5ZHWyL7tVUrmRPjrBrvyCb/go-ipfs-files"
	logging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"

	"github.com/filecoin-project/go-filecoin/protocol/storage"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
	localplugin "github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin/local"
)

func init() {
	// Enabling debug logging provides a lot of insight into what commands are
	// being executed
	logging.SetDebugLogging()
}

// TestRetrieval exercises storing and retreiving with the filecoin protocols
func TestRetrieval(t *testing.T) {
	t.SkipNow()

	// This test should run in 500 seconds, and no longer
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second*500))
	defer cancel()

	require := require.New(t)

	// Create a directory for the test using the test name (mostly for FAST)
	dir, err := ioutil.TempDir("", t.Name())
	require.NoError(err)

	// Create an environment that includes a genesis block with 1MM FIL
	env, err := fast.NewEnvironmentMemoryGenesis(big.NewInt(1000000), dir)
	require.NoError(err)

	// Teardown will shutdown all running processes the environment knows about
	// and cleanup anything the evironment setup. This includes the directory
	// the environment was created to use.
	defer env.Teardown(ctx)

	// Setup options for nodes.
	options := make(map[string]string)
	options[localplugin.AttrLogJSON] = "1"                               // Enable JSON logs
	options[localplugin.AttrLogLevel] = "5"                              // Set log level to Debug
	options[localplugin.AttrUseSmallSectors] = "true"                    // Enable small sectors
	options[localplugin.AttrFilecoinBinary] = th.MustGetFilecoinBinary() // Enable small sectors

	// Setup nodes used for the test
	genesis, err := env.NewProcess(ctx, localplugin.PluginName, options, fast.EnvironmentOpts{})
	require.NoError(err)

	miner, err := env.NewProcess(ctx, localplugin.PluginName, options, fast.EnvironmentOpts{})
	require.NoError(err)

	client, err := env.NewProcess(ctx, localplugin.PluginName, options, fast.EnvironmentOpts{})
	require.NoError(err)

	// Start setting up the nodes
	genesisURI := env.GenesisCar()
	genesisMiner, err := env.GenesisMiner()
	require.NoError(err)

	// Setup Genesis
	err = series.SetupGenesisNode(ctx, genesis, genesisURI, genesisMiner.Address, files.NewReaderFile(genesisMiner.Owner))
	require.NoError(err)

	// Start Miner
	_, err = miner.InitDaemon(ctx, "--genesisfile", genesisURI)
	require.NoError(err)

	_, err = miner.StartDaemon(ctx, true)
	require.NoError(err)

	// Start Client
	_, err = client.InitDaemon(ctx, "--genesisfile", genesisURI)
	require.NoError(err)

	_, err = client.StartDaemon(ctx, true)
	require.NoError(err)

	// Connect everything to the genesis node so it can issue filecoin when needed
	err = series.Connect(ctx, genesis, miner)
	require.NoError(err)

	err = series.Connect(ctx, genesis, client)
	require.NoError(err)

	// Setup the miner
	minerAddrs, err := miner.AddressLs(ctx)
	require.NoError(err)
	require.NotEmpty(minerAddrs, "no addresses for miner")

	// Sends filecoin from the genesis default wallet to the address provided, value 1000
	err = series.SendFilecoinFromDefault(ctx, genesis, minerAddrs[0], 1000)
	require.NoError(err)

	// Create a miner on the miner node
	_, err = miner.MinerCreate(ctx, 10, big.NewInt(10), fast.AOPrice(big.NewFloat(1.0)), fast.AOLimit(300))
	require.NoError(err)

	//TODO(tperson): I don't think a miner is valid unless it has power. Does
	// that mean we need to store before we can mine to be valid?

	err = miner.MiningStart(ctx)
	require.NoError(err)

	// Start retrieval
	// Start retrieval
	// Start retrieval

	// Set a price for our miner, and returns the created ask
	ask, err := series.SetPriceGetAsk(ctx, miner, big.NewFloat(1.0), big.NewInt(1000))
	require.NoError(err)

	// Connect the client and the miner
	err = series.Connect(ctx, miner, client)
	require.NoError(err)

	// Store some data with the miner with the given ask, returns the cid for
	// the imported data, and the deal which was created
	data := []byte("Hello World!")
	dataReader := bytes.NewReader(data)
	dcid, deal, err := series.ImportAndStore(ctx, client, ask, files.NewReaderFile(dataReader))
	require.NoError(err)

	// Wait for the deal to be posted
	err = series.WaitForDealState(ctx, client, deal, storage.Posted)
	require.NoError(err)

	// Retrieve the stored piece of data
	reader, err := client.RetrievalClientRetrievePiece(ctx, dcid, ask.Miner)
	require.NoError(err)

	// Verify that it's all the same
	retrievedData, err := ioutil.ReadAll(reader)
	require.NoError(err)
	require.Equal(data, retrievedData)
}
