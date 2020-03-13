package commands_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/fixtures"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestChainHead(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()
	builder := test.NewNodeBuilder(t)

	_, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	jsonResult := cmdClient.RunSuccess(ctx, "chain", "head", "--enc", "json").ReadStdoutTrimNewlines()
	var cidsFromJSON []cid.Cid
	err := json.Unmarshal([]byte(jsonResult), &cidsFromJSON)
	assert.NoError(t, err)

	textResult := cmdClient.RunSuccess(ctx, "chain", "ls", "--enc", "text").ReadStdoutTrimNewlines()
	textCid, err := cid.Decode(textResult)
	require.NoError(t, err)
	assert.Equal(t, textCid, cidsFromJSON[0])
}

func TestChainLs(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()
	t.Skip("DRAGONS: fake post for integration test")

	t.Run("chain ls with json encoding returns the whole chain as json", func(t *testing.T) {
		builder := test.NewNodeBuilder(t)
		buildWithMiner(t, builder)

		n, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		blk, err := n.BlockMining.BlockMiningAPI.MiningOnce(ctx)
		require.NoError(t, err)
		c := blk.Cid()

		result2 := cmdClient.RunSuccess(ctx, "chain", "ls", "--enc", "json").ReadStdoutTrimNewlines()
		var bs [][]block.Block
		for _, line := range bytes.Split([]byte(result2), []byte{'\n'}) {
			var b []block.Block
			err := json.Unmarshal(line, &b)
			require.NoError(t, err)
			bs = append(bs, b)
			require.Equal(t, 1, len(b))
		}

		assert.Equal(t, 2, len(bs))
		assert.True(t, bs[1][0].Parents.Empty())
		assert.True(t, c.Equals(bs[0][0].Cid()))
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

	t.Run("chain ls with text encoding returns only CIDs", func(t *testing.T) {
		builder := test.NewNodeBuilder(t)
		buildWithMiner(t, builder)

		n, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		var blocks []block.Block
		blockJSON := cmdClient.RunSuccess(ctx, "chain", "ls", "--enc", "json").ReadStdoutTrimNewlines()
		err := json.Unmarshal([]byte(blockJSON), &blocks)
		genesisBlockCid := blocks[0].Cid().String()
		require.NoError(t, err)

		blk, err := n.BlockMining.BlockMiningAPI.MiningOnce(ctx)
		require.NoError(t, err)
		newBlockCid := blk.Cid()

		expectedOutput := fmt.Sprintf("%s\n%s", newBlockCid, genesisBlockCid)

		chainLsResult := cmdClient.RunSuccess(ctx, "chain", "ls").ReadStdoutTrimNewlines()

		assert.Equal(t, chainLsResult, expectedOutput)
	})

	t.Run("chain ls --long returns CIDs, Miner, block height and message count", func(t *testing.T) {
		builder := test.NewNodeBuilder(t)
		buildWithMiner(t, builder)

		n, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		blk, err := n.BlockMining.BlockMiningAPI.MiningOnce(ctx)
		require.NoError(t, err)
		newBlockCid := blk.Cid().String()

		chainLsResult := cmdClient.RunSuccess(ctx, "chain", "ls", "--long").ReadStdoutTrimNewlines()

		assert.Contains(t, chainLsResult, newBlockCid)
		assert.Contains(t, chainLsResult, fixtures.TestMiners[0])
		assert.Contains(t, chainLsResult, "1")
		assert.Contains(t, chainLsResult, "0")
	})

	t.Run("chain ls --long with JSON encoding returns integer string block height", func(t *testing.T) {
		builder := test.NewNodeBuilder(t)
		buildWithMiner(t, builder)

		n, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		_, err := n.BlockMining.BlockMiningAPI.MiningOnce(ctx)
		require.NoError(t, err)

		chainLsResult := cmdClient.RunSuccess(ctx, "chain", "ls", "--long", "--enc", "json").ReadStdoutTrimNewlines()
		assert.Contains(t, chainLsResult, `"height":"0"`)
		assert.Contains(t, chainLsResult, `"height":"1"`)
	})
}
