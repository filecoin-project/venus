package commands_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/fastesting"
)

func TestInspectConfig(t *testing.T) {
	tf.IntegrationTest(t)

	ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fast.FilecoinOpts{})

	// Teardown after test ends
	defer func() {
		err := env.Teardown(ctx)
		require.NoError(t, err)
	}()

	nd := env.RequireNewNodeStarted()

	icfg, err := nd.InspectConfig(ctx)
	require.NoError(t, err)

	rcfg, err := nd.Config()
	require.NoError(t, err)

	// API is not the same since the FAST plugin reads the config file from disk,
	// this is a limitation of FAST.
	// FAST sets API.Address to /ip4/0.0.0.0/0 in the config file to avoid port collisions.
	// The Inspector returns an in memory representation of the config that has
	// received a port assignment from the kernel, meaning it is no longer /ip4/0.0.0.0/0
	assert.Equal(t, rcfg.Bootstrap, icfg.Bootstrap)
	assert.Equal(t, rcfg.Datastore, icfg.Datastore)
	assert.Equal(t, rcfg.Swarm, icfg.Swarm)
	assert.Equal(t, rcfg.Mining, icfg.Mining)
	assert.Equal(t, rcfg.Wallet, icfg.Wallet)
	assert.Equal(t, rcfg.Heartbeat, icfg.Heartbeat)
	assert.Equal(t, rcfg.Net, icfg.Net)
	assert.Equal(t, rcfg.Mpool, icfg.Mpool)
}
