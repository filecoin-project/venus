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

func TestConfigDaemon(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()

	t.Run("config <key> prints config value", func(t *testing.T) {
		builder := test.NewNodeBuilder(t)

		_, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		wrapped1 := config.NewDefaultConfig().Datastore

		var decodedOutput1 config.DatastoreConfig
		cmdClient.RunMarshaledJSON(ctx, &decodedOutput1, "config", "datastore")
		assert.Equal(t, wrapped1, &decodedOutput1)

		var path string
		cmdClient.RunMarshaledJSON(ctx, &path, "config", "datastore.path")
		assert.Equal(t, config.NewDefaultConfig().Datastore.Path, path)
	})

	t.Run("config <key> simple_value updates config", func(t *testing.T) {
		builder := test.NewNodeBuilder(t)

		n, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		period := "1m"
		// check writing default does not error
		cmdClient.RunSuccess(ctx, "config", "bootstrap.period", period)

		// validate output
		var retrievedPeriod string
		cmdClient.RunMarshaledJSON(ctx, &retrievedPeriod, "config", "bootstrap.period")
		assert.Equal(t, period, retrievedPeriod)

		// validate config write
		nbci, err := n.PorcelainAPI.ConfigGet("bootstrap.period")
		require.NoError(t, err)
		nbc, ok := nbci.(string)
		require.True(t, ok)
		assert.Equal(t, nbc, period)
	})

	t.Run("config <key> <val> updates config", func(t *testing.T) {
		builder := test.NewNodeBuilder(t)

		n, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		cmdClient.RunSuccess(ctx, "config", "bootstrap", `{"addresses": ["fake1", "fake2"], "period": "1m", "minPeerThreshold": 0}`)

		var bootstrapConfig config.BootstrapConfig
		cmdClient.RunMarshaledJSON(ctx, &bootstrapConfig, "config", "bootstrap")

		// validate output
		require.Len(t, bootstrapConfig.Addresses, 2)
		assert.Equal(t, "fake1", bootstrapConfig.Addresses[0])
		assert.Equal(t, "fake2", bootstrapConfig.Addresses[1])

		// validate config write
		nbci, err := n.PorcelainAPI.ConfigGet("bootstrap")
		require.NoError(t, err)
		nbc, ok := nbci.(*config.BootstrapConfig)
		require.True(t, ok)

		assert.Equal(t, nbc, &bootstrapConfig)
	})
}
