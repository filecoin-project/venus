package commands_test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/fastesting"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestChainHead(t *testing.T) {
	tf.IntegrationTest(t)

	ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fast.FilecoinOpts{})

	// Teardown after test ends
	defer func() {
		err := env.Teardown(ctx)
		require.NoError(t, err)
	}()

	chainDaemon := env.GenesisMiner
	_, err := chainDaemon.MiningOnce(ctx)
	assert.NoError(t, err)

	miningCid, err := chainDaemon.MiningOnce(ctx)
	assert.NoError(t, err)

	headResult, err := chainDaemon.ChainHead(ctx)
	assert.NoError(t, err)

	decoder, err := chainDaemon.ChainLs(ctx)
	assert.NoError(t, err)
	var blks []types.Block
	err = decoder.Decode(&blks)
	assert.NoError(t, err)
	assert.Equal(t, headResult[0], blks[0].Cid())
	assert.Equal(t, headResult[0], miningCid)
}

func TestChainLs(t *testing.T) {
	tf.IntegrationTest(t)

	t.Run("chain ls with json encoding returns the whole chain as json", func(t *testing.T) {
		ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fast.FilecoinOpts{})

		// Teardown after test ends
		defer func() {
			err := env.Teardown(ctx)
			require.NoError(t, err)
		}()

		chainDaemon := env.GenesisMiner
		c, err := chainDaemon.MiningOnce(ctx)
		assert.NoError(t, err)

		var bs [][]types.Block
		decoder, err := chainDaemon.ChainLs(ctx)
		assert.NoError(t, err)
		for {
			var blks []types.Block
			err = decoder.Decode(&blks)
			if err == io.EOF {
				break
			}
			assert.NoError(t, err)
			assert.Equal(t, 1, len(blks))
			bs = append(bs, blks)
		}

		assert.Equal(t, 2, len(bs))
		assert.True(t, bs[1][0].Parents.Empty())
		assert.True(t, c.Equals(bs[0][0].Cid()))
	})

	t.Run("chain ls with chain of size 1 returns genesis block", func(t *testing.T) {
		ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fast.FilecoinOpts{})

		// Teardown after test ends
		defer func() {
			err := env.Teardown(ctx)
			require.NoError(t, err)
		}()

		chainDaemon := env.GenesisMiner

		decoder, err := chainDaemon.ChainLs(ctx)
		assert.NoError(t, err)
		var blks []types.Block
		err = decoder.Decode(&blks)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(blks))

		assert.True(t, blks[0].Parents.Empty())
	})

	t.Run("chain ls with text encoding returns only CIDs", func(t *testing.T) {
		ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fast.FilecoinOpts{})

		// Teardown after test ends
		defer func() {
			err := env.Teardown(ctx)
			require.NoError(t, err)
		}()

		chainDaemon := env.GenesisMiner

		decoder, err := chainDaemon.ChainLs(ctx)
		assert.NoError(t, err)
		var blks []types.Block
		err = decoder.Decode(&blks)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(blks))
		genesisBlockCid := blks[0].Cid().String()

		newCid, err := chainDaemon.MiningOnce(ctx)

		expectedOutput := fmt.Sprintf("%s\n%s", newCid, genesisBlockCid)

		decoder, err = chainDaemon.ChainLs(ctx)
		assert.NoError(t, err)
		var chainLsResult []string
		for {
			var blks []types.Block
			err = decoder.Decode(&blks)
			if err == io.EOF {
				break
			}
			assert.NoError(t, err)
			assert.Equal(t, 1, len(blks))
			chainLsResult = append(chainLsResult, blks[0].Cid().String())
		}

		assert.Equal(t, strings.Join(chainLsResult, "\n"), expectedOutput)
	})

	t.Run("chain ls --long returns CIDs, Miner, block height and message count", func(t *testing.T) {
		ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fast.FilecoinOpts{})

		// Teardown after test ends
		defer func() {
			err := env.Teardown(ctx)
			require.NoError(t, err)
		}()

		chainDaemon := env.GenesisMiner

		newCid, err := chainDaemon.MiningOnce(ctx)
		assert.NoError(t, err)

		decoder, err := chainDaemon.ChainLs(ctx)
		assert.NoError(t, err)
		var chainLsResult [][]types.Block
		for {
			var blks []types.Block
			err = decoder.Decode(&blks)
			if err == io.EOF {
				break
			}
			assert.NoError(t, err)
			assert.Equal(t, 1, len(blks))
			chainLsResult = append(chainLsResult, blks)
		}
		minerConfig, err := chainDaemon.Config()
		assert.NoError(t, err)

		assert.Equal(t, chainLsResult[0][0].Cid(), newCid)
		assert.Equal(t, chainLsResult[0][0].Miner.String(), minerConfig.Mining.MinerAddress.String())
		assert.Equal(t, chainLsResult[0][0].Height, types.Uint64(1))
		assert.Equal(t, chainLsResult[1][0].Height, types.Uint64(0))
	})

	t.Run("chain ls --long with JSON encoding returns integer string block height and nonce", func(t *testing.T) {
		ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fast.FilecoinOpts{})

		// Teardown after test ends
		defer func() {
			err := env.Teardown(ctx)
			require.NoError(t, err)
		}()

		chainDaemon := env.GenesisMiner

		_, err := chainDaemon.MiningOnce(ctx)
		assert.NoError(t, err)
		decoder, err := chainDaemon.ChainLs(ctx)
		assert.NoError(t, err)
		var chainLsResult [][]types.Block
		for {
			var blks []types.Block
			err = decoder.Decode(&blks)
			if err == io.EOF {
				break
			}
			assert.NoError(t, err)
			assert.Equal(t, 1, len(blks))
			chainLsResult = append(chainLsResult, blks)
		}

		assert.Equal(t, chainLsResult[1][0].Height, types.Uint64(0))
		assert.Equal(t, chainLsResult[0][0].Height, types.Uint64(1))
		assert.Equal(t, chainLsResult[0][0].Nonce, types.Uint64(0))
	})
}

func TestChainBlock(t *testing.T) {
	tf.IntegrationTest(t)

	t.Run("chain block <cid-of-genesis-block> --enc=json returns output for the filecoin block", func(t *testing.T) {
		ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fast.FilecoinOpts{})

		// Teardown after test ends
		defer func() {
			err := env.Teardown(ctx)
			require.NoError(t, err)
		}()

		chainDaemon := env.GenesisMiner

		// mine a block and get its CID
		newCid, err := chainDaemon.MiningOnce(ctx)
		assert.NoError(t, err)

		// get the mined block by its CID
		outputBlock, err := chainDaemon.ChainBlock(ctx, newCid)
		assert.NoError(t, err)
		minerConfig, err := chainDaemon.Config()
		assert.NoError(t, err)

		assert.Equal(t, outputBlock.ParentWeight, types.Uint64(0))
		assert.Equal(t, outputBlock.Height, types.Uint64(1))
		assert.Equal(t, outputBlock.Nonce, types.Uint64(0))
		assert.Equal(t, outputBlock.Miner.String(), minerConfig.Mining.MinerAddress.String())
		assert.Equal(t, outputBlock.Messages[0].MeteredMessage.Message.To.String(),
			minerConfig.Mining.MinerAddress.String())
		assert.Equal(t, outputBlock.Messages[0].MeteredMessage.Message.Method, "updatePeerID")
	})
}
