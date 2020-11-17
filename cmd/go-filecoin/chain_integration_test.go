package commands_test

import (
	"bytes"
	"context"
	"encoding/json"
	commands "github.com/filecoin-project/venus/cmd/go-filecoin"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/internal/app/go-filecoin/node/test"

	"github.com/filecoin-project/venus/internal/pkg/block"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
)

func TestChainHead(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()
	builder := test.NewNodeBuilder(t)

	_, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	jsonResult := cmdClient.RunSuccess(ctx, "chain", "head", "--enc", "json").ReadStdoutTrimNewlines()
	var cidsFromJSON commands.ChainHeadResult
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
		var bs [][]block.Block
		for _, line := range bytes.Split([]byte(result2), []byte{'\n'}) {
			var b []block.Block
			err := json.Unmarshal(line, &b)
			require.NoError(t, err)
			bs = append(bs, b)
			require.Equal(t, 1, len(b))
		}

		assert.Equal(t, 1, len(bs))
		assert.True(t, bs[0][0].Parents.Empty())
	})

	t.Run("chain ls with chain of size 1 returns genesis block", func(t *testing.T) {
		builder := test.NewNodeBuilder(t)

		_, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		op := cmdClient.RunSuccess(ctx, "chain", "ls", "--enc", "json")
		result := op.ReadStdoutTrimNewlines()

		var b []block.Block
		err := json.Unmarshal([]byte(result), &b)
		require.NoError(t, err)

		assert.True(t, b[0].Parents.Empty())
	})

	//t.Run("chain ls --long returns CIDs, Miner, block height and message count", func(t *testing.T) {
	//	seed, cfg, chainClk := test.CreateBootstrapSetup(t)
	//	n := test.CreateBootstrapMiner(ctx, t, seed, chainClk, cfg)
	//
	//	cmdClient, apiDone := test.RunNodeAPI(ctx, n, t)
	//	defer apiDone()
	//
	//	chainLsResult := cmdClient.RunSuccess(ctx, "chain", "ls", "--long").ReadStdoutTrimNewlines()
	//
	//	assert.Contains(t, chainLsResult, fortest.TestMiners[0].String())
	//	assert.Contains(t, chainLsResult, "1")
	//	assert.Contains(t, chainLsResult, "0")
	//})
	//
	//t.Run("chain ls --long with JSON encoding returns integer string block height", func(t *testing.T) {
	//	seed, cfg, chainClk := test.CreateBootstrapSetup(t)
	//	n := test.CreateBootstrapMiner(ctx, t, seed, chainClk, cfg)
	//
	//	cmdClient, apiDone := test.RunNodeAPI(ctx, n, t)
	//	defer apiDone()
	//
	//	chainLsResult := cmdClient.RunSuccess(ctx, "chain", "ls", "--long", "--enc", "json").ReadStdoutTrimNewlines()
	//	assert.Contains(t, chainLsResult, `"height":0`)
	//	assert.Contains(t, chainLsResult, `"height":1`)
	//})
}
