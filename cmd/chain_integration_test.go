package cmd_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/app/node/test"
	"github.com/filecoin-project/venus/cmd"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func TestChainHead(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()
	builder := test.NewNodeBuilder(t)

	_, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	jsonResult := cmdClient.RunSuccess(ctx, "chain", "head", "--enc", "json").ReadStdoutTrimNewlines()
	var cidsFromJSON cmd.ChainHeadResult
	err := json.Unmarshal([]byte(jsonResult), &cidsFromJSON)
	assert.NoError(t, err)
}

func TestChainLs(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()

	t.Run("chain ls returns the specified number of tipsets modified by the count", func(t *testing.T) {
		seed, cfg, chainClk := test.CreateBootstrapSetup(t)
		n := test.CreateBootstrapMiner(ctx, t, seed, chainClk, cfg)

		cmdClient, apiDone := test.RunNodeAPI(ctx, n, t)
		defer apiDone()

		result := cmdClient.RunSuccess(ctx, "chain", "ls", "--count", "2").ReadStdoutTrimNewlines()
		rows := strings.Count(result, "\n")
		require.Equal(t, rows, 0)
	})
}
