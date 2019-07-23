Network Deployment Tests
========================

These tests can be run against a deployed kittyhawk network to verify
expected behavior.

All tests can be run by invoking the `test` command with the `-deployment <network>`
flag from the project root.

```
$ go run ./build test -deployment nightly -unit false
```

## Writing Tests

A deployment test is any test with a call to `tf.Deployment(t)`. The call to `tf.Deployment`
returns the network name the test should be configured for. This value can be passed into
`environment.NewDevnet`, and `fast.PODevnet` so everything is configured correctly.

Due to the large setup cost (chain syncing / processing) using the same process is desired for
related tests as long as no state between tests is shared (creating a miner in one, and using it
in another).

```
package networkdeployment_test

import (
	"context"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/environment"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
	localplugin "github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin/local"
)

func TestFoo(t *testing.T) {
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

	node, err := env.NewProcess(ctx, localplugin.PluginName, options, fastenvOpts)
	require.NoError(t, err)

	err = series.InitAndStart(ctx, node)
	require.NoError(t, err)

  // Tests can reuse the same enviroment or even a shared process. Tests should not depend
  // on output of any other test such as a created miner, etc

  t.Run("Do something with node", func(t *testing.T) {
  })

  t.Run("Do something else with node", func(t *testing.T) {
  })
```
