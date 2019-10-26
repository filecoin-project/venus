Network Deployment Tests
========================

These tests can be run against a deployed kittyhawk network to verify
expected behavior.

All tests can be run by invoking the `test` command with the `-deployment <network>`
flag from the project root. A `go-filecoin` binary should be built and located in the
project root (`go run ./build build-filecoin`).

```
$ go run ./build test -deployment nightly -unit false
```

Their are currently four suported networks: `nightly`, `staging`, `users`, and `local`.

The `local` network is primarly used to help during development as the network will run
with a `5s` block time and smaller 1KiB sectors.

## Writing Tests

A deployment test is any test with a call to `tf.Deployment(t)`. The call to `tf.Deployment`
returns the network name the test should be configured for. This value can be passed into
`fastesting.NewDeploymentEnvironment` so everything is configured correctly.

Due to the large setup cost (chain syncing / processing) the same process can be used for
related tests as long as no state between tests is shared (creating a miner in one, and using it
in another).

```
package networkdeployment_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/fastesting"
)

func TestFoo(t *testing.T) {
	network := tf.DeploymentTest(t)

	ctx := context.Background()
	ctx, env := fastesting.NewDeploymentEnvironment(ctx, t, network, fast.FilecoinOpts{})

	// Teardown will shutdown all running processes the environment knows about
	// and cleanup anything the environment setup. This includes the directory
	// the environment was created to use.
	defer func() {
		err := env.Teardown(ctx)
		require.NoError(t, err)
	}()

	node := env.RequireNewNodeWithFunds()

	 // Tests can reuse the same enviroment or even a shared process. Tests should not depend
	 // on output of any other test such as a created miner, etc

	 t.Run("Do something with node", func(t *testing.T) {
	 })

	 t.Run("Do something else with node", func(t *testing.T) {
	 })
```
