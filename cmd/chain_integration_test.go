package cmd_test

import (
	"bytes"
	"context"
	"encoding/json"
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

	t.Run("chain ls with json encoding returns the whole chain as json", func(t *testing.T) {
		seed, cfg, chainClk := test.CreateBootstrapSetup(t)
		n := test.CreateBootstrapMiner(ctx, t, seed, chainClk, cfg)

		cmdClient, apiDone := test.RunNodeAPI(ctx, n, t)
		defer apiDone()

		result2 := cmdClient.RunSuccess(ctx, "chain", "ls", "--enc", "json").ReadStdoutTrimNewlines()
		var bs [][]cmd.ChainLsResult
		for _, line := range bytes.Split([]byte(result2), []byte{'\n'}) {
			var b []cmd.ChainLsResult
			err := json.Unmarshal(line, &b)
			require.NoError(t, err)
			bs = append(bs, b)
			require.Equal(t, 1, len(b))
		}

		assert.Equal(t, 1, len(bs))
	})

	t.Run("chain ls with chain of size 1 returns genesis block", func(t *testing.T) {
		builder := test.NewNodeBuilder(t)

		_, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		op := cmdClient.RunSuccess(ctx, "chain", "ls", "--enc", "json")
		result := op.ReadStdoutTrimNewlines()

		var b []cmd.ChainLsResult
		err := json.Unmarshal([]byte(result), &b)
		require.NoError(t, err)
	})
}
