package commands_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/node/test"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

func TestProtocol(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()

	// Create node (it's not necessary to start it).
	b := test.NewNodeBuilder(t)
	node := b.
		WithConfig(func(c *config.Config) {
			c.Mining.AutoSealIntervalSeconds = 120
		}).
		Build(ctx)
	require.NoError(t, node.ChainReader.Load(ctx))

	// Run the command API.
	cmd, stop := test.RunNodeAPI(ctx, node, t)
	defer stop()

	out := cmd.RunSuccess(ctx, "protocol").ReadStdout()
	assert.Contains(t, out, "Network: go-filecoin-test")
	assert.Contains(t, out, "Auto-Seal Interval: 120 seconds")
}
