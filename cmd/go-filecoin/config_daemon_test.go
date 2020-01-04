package commands_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestConfigDaemon(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()

	t.Run("config <key> prints config value", func(t *testing.T) {
		builder := test.NewNodeBuilder(t)

		n := builder.BuildAndStart(ctx)
		defer n.Stop(ctx)
		cmdClient, done := test.RunNodeAPI(ctx, n, t)
		defer done()

		op1 := cmdClient.RunSuccess(ctx, "config", "datastore")
		jsonOut := op1.ReadStdout()
		wrapped1 := config.NewDefaultConfig().Datastore
		var decodedOutput1 config.DatastoreConfig
		err := json.Unmarshal([]byte(jsonOut), &decodedOutput1)
		require.NoError(t, err)
		assert.Equal(t, wrapped1, &decodedOutput1)

		op2 := cmdClient.RunSuccess(ctx, "config", "datastore.path")
		jsonOut = op2.ReadStdout()
		assert.Equal(t, fmt.Sprintf("%q\n", config.NewDefaultConfig().Datastore.Path), jsonOut)
	})

	t.Run("config <key> simple_value updates config", func(t *testing.T) {
		builder := test.NewNodeBuilder(t)

		n := builder.BuildAndStart(ctx)
		defer n.Stop(ctx)
		cmdClient, done := test.RunNodeAPI(ctx, n, t)
		defer done()

		period := "1m"
		// check writing default does not error
		cmdClient.RunSuccess(ctx, "config", "bootstrap.period", period)
		op1 := cmdClient.RunSuccess(ctx, "config", "bootstrap.period")

		// validate output
		jsonOut := op1.ReadStdout()
		bootstrapConfig := config.NewDefaultConfig().Bootstrap
		assert.Equal(t, fmt.Sprintf("\"%s\"\n", period), jsonOut)

		// validate config write
		nbci, err := n.PorcelainAPI.ConfigGet("bootstrap")
		require.NoError(t, err)
		nbc, ok := nbci.(*config.BootstrapConfig)
		require.True(t, ok)
		assert.Equal(t, nbc, bootstrapConfig)
	})

	t.Run("config <key> <val> updates config", func(t *testing.T) {
		builder := test.NewNodeBuilder(t)

		n := builder.BuildAndStart(ctx)
		defer n.Stop(ctx)
		cmdClient, done := test.RunNodeAPI(ctx, n, t)
		defer done()

		cmdClient.RunSuccess(ctx, "config", "bootstrap", `{"addresses": ["fake1", "fake2"], "period": "1m", "minPeerThreshold": 0}`)
		op1 := cmdClient.RunSuccess(ctx, "config", "bootstrap")

		// validate output
		jsonOut := op1.ReadStdout()
		bootstrapConfig := config.NewDefaultConfig().Bootstrap
		bootstrapConfig.Addresses = []string{"fake1", "fake2"}
		someJSON, err := json.MarshalIndent(bootstrapConfig, "", "\t")
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("%s\n", string(someJSON)), jsonOut)

		// validate config write
		nbci, err := n.PorcelainAPI.ConfigGet("bootstrap")
		require.NoError(t, err)
		nbc, ok := nbci.(*config.BootstrapConfig)
		require.True(t, ok)

		assert.Equal(t, nbc, bootstrapConfig)
	})
}
