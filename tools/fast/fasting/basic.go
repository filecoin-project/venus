package fasting

import (
	"context"
	"io/ioutil"
	"math/big"
	"strings"
	"testing"

	"github.com/ipfs/go-ipfs-files"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
	localplugin "github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin/local"
)

// BasicFastSetup creates a environment and a single node, and environment options
func BasicFastSetup(ctx context.Context, t *testing.T, fastenvOpts fast.EnvironmentOpts) (fast.Environment, *fast.Filecoin, func() *fast.Filecoin, context.Context) {
	require := require.New(t)

	// Create a directory for the test using the test name (mostly for FAST)
	// TODO(tperson) use a different TempDir that uses MkdirAll
	dir, err := ioutil.TempDir("", strings.Replace(t.Name(), "/", ".", -1))
	require.NoError(err)

	// Create an environment that includes a genesis block with 1MM FIL
	env, err := fast.NewEnvironmentMemoryGenesis(big.NewInt(1000000), dir)
	require.NoError(err)

	// Setup options for nodes.
	options := make(map[string]string)
	options[localplugin.AttrLogJSON] = "1"                                        // Enable JSON logs
	options[localplugin.AttrLogLevel] = "5"                                       // Set log level to Debug
	options[localplugin.AttrUseSmallSectors] = "true"                             // Enable small sectors
	options[localplugin.AttrFilecoinBinary] = testhelpers.MustGetFilecoinBinary() // Get the filecoin binary

	genesisURI := env.GenesisCar()
	genesisMiner, err := env.GenesisMiner()
	require.NoError(err)

	fastenvOpts = fast.EnvironmentOpts{
		InitOpts:   append([]fast.ProcessInitOption{fast.POGenesisFile(genesisURI)}, fastenvOpts.InitOpts...),
		DaemonOpts: append([]fast.ProcessDaemonOption{fast.POBlockTime(series.GlobalSleepDelay)}, fastenvOpts.DaemonOpts...),
	}

	// Create a node for the test
	genesis, err := env.NewProcess(ctx, localplugin.PluginName, options, fastenvOpts)
	require.NoError(err)

	err = series.SetupGenesisNode(ctx, genesis, genesisMiner.Address, files.NewReaderFile(genesisMiner.Owner))
	require.NoError(err)

	err = genesis.MiningStart(ctx)
	require.NoError(err)

	NewNode := func() *fast.Filecoin {
		p, err := env.NewProcess(ctx, localplugin.PluginName, options, fastenvOpts)
		require.NoError(err)

		err = series.InitAndStart(ctx, p)
		require.NoError(err)

		err = series.Connect(ctx, genesis, p)
		require.NoError(err)

		err = series.SendFilecoinDefaults(ctx, genesis, p, 10000)
		require.NoError(err)

		return p
	}

	return env, genesis, NewNode, ctx
}
