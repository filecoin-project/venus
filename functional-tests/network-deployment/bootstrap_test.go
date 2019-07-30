package networkdeployment_test

import (
	"context"
	"io"
	"io/ioutil"
	"testing"
	"time"

	//logging "github.com/ipfs/go-log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/fixtures"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/environment"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
	localplugin "github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin/local"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-circuit"
	"github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	notif "github.com/libp2p/go-libp2p-routing/notifications"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multiaddr-dns"
)

func init() {
	//logging.SetDebugLogging()
}

// TestBootstrap verifies information about the bootstrap peers
func TestBootstrap(t *testing.T) {
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

	client, err := env.NewProcess(ctx, localplugin.PluginName, options, fastenvOpts)
	require.NoError(t, err)

	err = series.InitAndStart(ctx, client)
	require.NoError(t, err)

	t.Run("Check that we are connected to bootstrap peers", func(t *testing.T) {
	})
}
