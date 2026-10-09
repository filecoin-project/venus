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

// TestChainGetBlockRaw covers the `--raw` option of `chain get-block`. The option must be
// honoured by its value, not merely by its presence: `--raw=false` has to behave exactly
// like omitting the flag.
func TestChainGetBlockRaw(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()
	builder := test.NewNodeBuilder(t)

	_, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	headJSON := cmdClient.RunSuccess(ctx, "chain", "head", "--enc", "json").ReadStdoutTrimNewlines()
	var head cmd.ChainHeadResult
	require.NoError(t, json.Unmarshal([]byte(headJSON), &head))
	require.NotEmpty(t, head.Cids)
	blockCid := head.Cids[0].String()

	// The non-raw output decorates the block header with the message and receipt sets,
	// which the raw output (a bare block header) does not carry.
	defaultOut := cmdClient.RunSuccess(ctx, "chain", "get-block", blockCid).ReadStdout()
	rawTrueOut := cmdClient.RunSuccess(ctx, "chain", "get-block", blockCid, "--raw=true").ReadStdout()
	rawFalseOut := cmdClient.RunSuccess(ctx, "chain", "get-block", blockCid, "--raw=false").ReadStdout()

	t.Run("without --raw the block is printed with its messages", func(t *testing.T) {
		assert.Contains(t, defaultOut, "BlsMessages")
		assert.Contains(t, defaultOut, "SecpkMessages")
	})

	t.Run("--raw=true prints only the block header", func(t *testing.T) {
		assert.NotContains(t, rawTrueOut, "BlsMessages")
		assert.NotContains(t, rawTrueOut, "SecpkMessages")
	})

	t.Run("--raw=false behaves like the default", func(t *testing.T) {
		assert.Contains(t, rawFalseOut, "BlsMessages", "--raw=false must not fall through to the raw branch")
		assert.Contains(t, rawFalseOut, "SecpkMessages")
		assert.Contains(t, rawFalseOut, "ParentMessages")
		assert.Equal(t, defaultOut, rawFalseOut)
	})
}
