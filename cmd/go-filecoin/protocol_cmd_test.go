package commands_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/internal/app/go-filecoin/node/test"
	"github.com/filecoin-project/venus/internal/pkg/config"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
)

func TestProtocol(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()

	// Create node (it's not necessary to start it).
	b := test.NewNodeBuilder(t)
	node := b.
		WithConfig(func(c *config.Config) {

		}).
		Build(ctx)
	require.NoError(t, node.Chain().ChainReader.Load(ctx))

	// Run the command API.
	cmd, stop := test.RunNodeAPI(ctx, node, t)
	defer stop()

	out := cmd.RunSuccess(ctx, "protocol").ReadStdout()
	assert.Contains(t, out, "\"Network\": \"gfctest\"")
}
