package tests

import (
	"context"
	"io/ioutil"
	"math/big"
	"testing"
	"time"

	"github.com/ipfs/go-ipfs-files"
	logging "github.com/ipfs/go-log"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/chain"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/environment"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
	localplugin "github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin/local"
	"github.com/filecoin-project/go-filecoin/types"
)

// modify config to decrease catchup period and sets all nodes to trust eachother.
var trustCatchupCfg = func(ctx context.Context, node *fast.Filecoin) error {
	cfg, err := node.Config()
	if err != nil {
		return err
	}
	cfg.Sync.TrustAllPeers = true
	cfg.Sync.CatchupSyncerPeriod = "3s"
	if err := node.WriteConfig(cfg); err != nil {
		return err
	}
	return nil
}

func init() {
	// Enabling debug logging provides a lot of insight into what commands are
	// being executed
	logging.SetAllLoggers(3)
}

func TestChainCatchupSyncLocalNetwork(t *testing.T) {
	tf.FunctionalTest(t)

	// mine blocks quickly to get a large chain weight.
	blocktime := time.Millisecond * 10
	// this deadline was picked based on trial and error.
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(90*time.Second))
	defer cancel()

	dir, err := ioutil.TempDir("", t.Name())
	require.NoError(t, err)

	env, err := environment.NewMemoryGenesis(big.NewInt(1000000), dir, types.TestProofsMode)
	require.NoError(t, err)
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

	// Setup Genesis, it will trust all peers since it is the first miner.
	err = series.SetupGenesisNode(ctx, genesis, genesisMiner.Address, files.NewReaderFile(genesisMiner.Owner), trustCatchupCfg)
	require.NoError(t, err)

	// make a dummy node to get the genesis node out of catchup mode.
	// genesis will connect to this node, compare its head, see that its ahead and exit chain catchup
	dummyNode, err := env.NewProcess(ctx, localplugin.PluginName, options, fastenvOpts)
	require.NoError(t, err)
	require.NoError(t, series.InitAndStart(ctx, dummyNode, trustCatchupCfg))
	require.NoError(t, series.Connect(ctx, genesis, dummyNode))

	client, err := env.NewProcess(ctx, localplugin.PluginName, options, fastenvOpts)
	require.NoError(t, err)

	genesisPeerDetails, err := genesis.ID(ctx)
	require.NoError(t, err)

	// Start Client configure it to trust the genesis node.
	err = series.InitAndStart(ctx, client, func(ctx context.Context, node *fast.Filecoin) error {
		cfg, err := node.Config()
		if err != nil {
			return err
		}
		for _, addr := range genesisPeerDetails.Addresses {
			cfg.Sync.TrustedAddresses = append(cfg.Sync.TrustedAddresses, addr.String())
		}
		cfg.Sync.CatchupSyncerPeriod = "3s"
		if err := node.WriteConfig(cfg); err != nil {
			return err
		}
		return nil
	})
	require.NoError(t, err)

	// mine until the chain is too long for an untrusted sync
	require.NoError(t, genesis.MiningStart(ctx))
	var networkHeight uint64
	for networkHeight < uint64(2*chain.UntrustedChainHeightLimit) {
		status, err := genesis.ChainStatus(ctx)
		require.NoError(t, err)
		networkHeight = status.ValidatedHeadHeight
	}
	// stop all mining on the network, given the short blocktime the client may never catchup.
	err = genesis.MiningStop(ctx)
	require.NoError(t, err)

	// connect the genesis and the client together. This will allow the client to begin a catchup sync.
	require.NoError(t, series.Connect(ctx, genesis, client))

	// client should reach the same height as the genesis node.
	genesisStatus, err := genesis.ChainStatus(ctx)
	require.NoError(t, err)
	// now ensure the client can catchup
	var clientHead types.TipSetKey
	for !clientHead.Equals(genesisStatus.ValidatedHead) {
		status, err := client.ChainStatus(ctx)
		require.NoError(t, err)
		clientHead = status.ValidatedHead
	}
}
