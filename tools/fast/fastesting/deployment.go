package fastesting

import (
	"context"
	"io/ioutil"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ipfs/go-ipfs-files"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/environment"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
	localplugin "github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin/local"
)

// DeploymentEnvironment provides common setup for writing tests which will run against
// a deployed network using FAST
type DeploymentEnvironment struct {
	environment.Environment

	t   *testing.T
	ctx context.Context

	pluginName string
	pluginOpts map[string]string

	postInitFn func(context.Context, *fast.Filecoin) error

	fastenvOpts fast.FilecoinOpts
}

// NewDeploymentEnvironment creates a DeploymentEnvironment with a basic setup for writing
// tests using the FAST library. DeploymentEnvironment also supports testing locally using
// the `local` network which will handle setting up a mining node and updating bootstrap
// peers. The local network runs at 5 second blocktimes.
func NewDeploymentEnvironment(ctx context.Context, t *testing.T, network string, fastenvOpts fast.FilecoinOpts) (context.Context, *DeploymentEnvironment) {

	// Create a directory for the test using the test name (mostly for FAST)
	// Replace the forward slash as tempdir can't handle them
	dir, err := ioutil.TempDir("", strings.Replace(t.Name(), "/", ".", -1))
	require.NoError(t, err)

	if network == "local" {
		return makeLocal(ctx, t, dir, fastenvOpts)
	}

	return makeDevnet(ctx, t, network, dir, fastenvOpts)
}

func makeLocal(ctx context.Context, t *testing.T, dir string, fastenvOpts fast.FilecoinOpts) (context.Context, *DeploymentEnvironment) {
	// Create an environment to connect to the devnet
	env, err := environment.NewMemoryGenesis(big.NewInt(1000000), dir, types.TestProofsMode)
	require.NoError(t, err)

	defer func() {
		dumpEnvOutputOnFail(t, env.Processes())
	}()

	// Setup options for nodes.
	options := make(map[string]string)
	options[localplugin.AttrLogJSON] = "0"
	options[localplugin.AttrLogLevel] = "5"
	options[localplugin.AttrFilecoinBinary] = testhelpers.MustGetFilecoinBinary()

	genesisURI := env.GenesisCar()
	genesisMiner, err := env.GenesisMiner()
	require.NoError(t, err)

	fastenvOpts.InitOpts = append([]fast.ProcessInitOption{fast.POGenesisFile(genesisURI)}, fastenvOpts.InitOpts...)
	fastenvOpts.DaemonOpts = append([]fast.ProcessDaemonOption{fast.POBlockTime(time.Second * 5)}, fastenvOpts.DaemonOpts...)

	ctx = series.SetCtxSleepDelay(ctx, time.Second*5)

	// Setup the first node which is used to help coordinate the other nodes by providing
	// funds, mining for the network, etc
	genesis, err := env.NewProcess(ctx, localplugin.PluginName, options, fastenvOpts)
	require.NoError(t, err)

	err = series.SetupGenesisNode(ctx, genesis, genesisMiner.Address, files.NewReaderFile(genesisMiner.Owner))
	require.NoError(t, err)

	err = genesis.MiningStart(ctx)
	require.NoError(t, err)

	details, err := genesis.ID(ctx)
	require.NoError(t, err)

	return ctx, &DeploymentEnvironment{
		Environment: env,
		t:           t,
		ctx:         ctx,
		pluginName:  localplugin.PluginName,
		pluginOpts:  options,
		fastenvOpts: fastenvOpts,
		postInitFn: func(ctx context.Context, node *fast.Filecoin) error {
			config, err := node.Config()
			if err != nil {
				return err
			}

			config.Bootstrap.Addresses = []string{details.Addresses[0].String()}
			config.Bootstrap.MinPeerThreshold = 1
			config.Bootstrap.Period = "10s"

			return node.WriteConfig(config)
		},
	}
}

func makeDevnet(ctx context.Context, t *testing.T, network string, dir string, fastenvOpts fast.FilecoinOpts) (context.Context, *DeploymentEnvironment) {
	// Create an environment that includes a genesis block with 1MM FIL
	networkConfig, err := environment.FindDevnetConfigByName(network)
	require.NoError(t, err)

	env, err := environment.NewDevnet(networkConfig, dir)
	require.NoError(t, err)

	defer func() {
		dumpEnvOutputOnFail(t, env.Processes())
	}()

	// Setup options for nodes.
	options := make(map[string]string)
	options[localplugin.AttrLogJSON] = "0"                                        // Enable JSON logs
	options[localplugin.AttrLogLevel] = "5"                                       // Set log level to Debug
	options[localplugin.AttrFilecoinBinary] = testhelpers.MustGetFilecoinBinary() // Get the filecoin binary

	genesisURI := env.GenesisCar()

	fastenvOpts.InitOpts = append(fastenvOpts.InitOpts, fast.POGenesisFile(genesisURI), fast.PODevnet(networkConfig.Name))

	ctx = series.SetCtxSleepDelay(ctx, time.Second*30)

	return ctx, &DeploymentEnvironment{
		Environment: env,
		t:           t,
		ctx:         ctx,
		pluginName:  localplugin.PluginName,
		pluginOpts:  options,
		fastenvOpts: fastenvOpts,
		postInitFn: func(ctx context.Context, node *fast.Filecoin) error {
			return nil
		},
	}
}

// RequireNewNodeStarted builds a new node using RequireNewNode, then initializes
// and starts it
func (env *DeploymentEnvironment) RequireNewNodeStarted() *fast.Filecoin {
	p, err := env.NewProcess(env.ctx, env.pluginName, env.pluginOpts, env.fastenvOpts)
	require.NoError(env.t, err)

	err = series.InitAndStart(env.ctx, p, env.postInitFn)
	require.NoError(env.t, err)

	return p
}

// RequireNewNodeWithFunds builds a new node using RequireNewNodeStarted, then
// sends it funds from the environment GenesisMiner node
func (env *DeploymentEnvironment) RequireNewNodeWithFunds() *fast.Filecoin {
	p := env.RequireNewNodeStarted()

	err := env.GetFunds(env.ctx, p)
	require.NoError(env.t, err)

	return p
}

// Teardown stops all of the nodes and cleans up the environment. If the test failed,
// it will also print the last output of each process by calling `DumpLastOutput`.
// Output is logged using the Log method on the testing.T
func (env *DeploymentEnvironment) Teardown(ctx context.Context) error {
	env.DumpEnvOutputOnFail()
	return env.Environment.Teardown(ctx)
}

// DumpEnvOutputOnFail calls `DumpLastOutput for each process if the test failed.
func (env *DeploymentEnvironment) DumpEnvOutputOnFail() {
	dumpEnvOutputOnFail(env.t, env.Processes())
}
