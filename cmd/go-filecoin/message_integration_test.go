package commands_test

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/fixtures/fortest"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

func TestMessageSend(t *testing.T) {
	t.Skip("DRAGONS: fake proofs")
	tf.IntegrationTest(t)
	ctx := context.Background()
	builder := test.NewNodeBuilder(t)
	defaultAddr := fortest.TestAddresses[0]

	cs := node.FixtureChainSeed(t)
	builder.WithGenesisInit(cs.GenesisInitFunc)
	builder.WithConfig(cs.MinerConfigOpt(0))
	builder.WithConfig(node.DefaultAddressConfigOpt(defaultAddr))
	builder.WithInitOpt(cs.KeyInitOpt(1))
	builder.WithInitOpt(cs.KeyInitOpt(0))

	n, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	_, err := n.BlockMining.BlockMiningAPI.MiningOnce(ctx)
	require.NoError(t, err)

	from, err := n.PorcelainAPI.WalletDefaultAddress() // this should = fixtures.TestAddresses[0]
	require.NoError(t, err)

	t.Log("[failure] invalid target")
	cmdClient.RunFail(
		ctx,
		address.ErrUnknownNetwork.Error(),
		"message", "send",
		"--from", from.String(),
		"--gas-price", "0", "--gas-limit", "300",
		"--value=10", "xyz",
	)

	t.Log("[success] with from")
	cmdClient.RunSuccess(
		ctx,
		"message", "send",
		"--from", from.String(),
		"--gas-price", "1",
		"--gas-limit", "300",
		fortest.TestAddresses[3].String(),
	)

	t.Log("[success] with from and int value")
	cmdClient.RunSuccess(
		ctx,
		"message", "send",
		"--from", from.String(),
		"--gas-price", "1",
		"--gas-limit", "300",
		"--value", "10",
		fortest.TestAddresses[3].String(),
	)

	t.Log("[success] with from and decimal value")
	cmdClient.RunSuccess(
		ctx,
		"message", "send",
		"--from", from.String(),
		"--gas-price", "1",
		"--gas-limit", "300",
		"--value", "5.5",
		fortest.TestAddresses[3].String(),
	)
}

func TestMessageWait(t *testing.T) {
	t.Skip("Dragons: fake proofs")
	tf.IntegrationTest(t)
	ctx := context.Background()
	builder := test.NewNodeBuilder(t)

	buildWithMiner(t, builder)
	n, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	t.Run("[success] transfer only", func(t *testing.T) {
		msg := cmdClient.RunSuccess(ctx, "message", "send",
			"--from", fortest.TestAddresses[0].String(),
			"--gas-price", "1",
			"--gas-limit", "300",
			fortest.TestAddresses[1].String(),
		)
		msgcid := msg.ReadStdoutTrimNewlines()

		// Fail with timeout before the message has been mined
		cmdClient.RunFail(
			ctx,
			"deadline exceeded",
			"message", "wait",
			"--message=false",
			"--receipt=false",
			"--timeout=100ms",
			"--return",
			msgcid,
		)

		_, err := n.BlockMining.BlockMiningAPI.MiningOnce(ctx)
		require.NoError(t, err)

		wait := cmdClient.RunSuccess(
			ctx,
			"message", "wait",
			"--message=false",
			"--receipt=false",
			"--timeout=1m",
			"--return",
			msgcid,
		)
		assert.Equal(t, "", wait.ReadStdout())
	})
}

func TestMessageSendBlockGasLimit(t *testing.T) {
	tf.IntegrationTest(t)
	t.Skip("Dragons: fake proofs")

	ctx := context.Background()
	builder := test.NewNodeBuilder(t)
	defaultAddr := fortest.TestAddresses[0]

	buildWithMiner(t, builder)
	builder.WithConfig(node.DefaultAddressConfigOpt(defaultAddr))
	n, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	doubleTheBlockGasLimit := strconv.Itoa(int(types.BlockGasLimit) * 2)
	halfTheBlockGasLimit := strconv.Itoa(int(types.BlockGasLimit) / 2)
	result := struct{ Messages types.TxMeta }{}

	t.Run("when the gas limit is above the block limit, the message fails", func(t *testing.T) {
		cmdClient.RunFail(
			ctx,
			"block gas limit",
			"message", "send",
			"--gas-price", "1", "--gas-limit", doubleTheBlockGasLimit,
			"--value=10", fortest.TestAddresses[1].String(),
		)
	})

	t.Run("when the gas limit is below the block limit, the message succeeds", func(t *testing.T) {
		cmdClient.RunSuccess(
			ctx,
			"message", "send",
			"--gas-price", "1", "--gas-limit", halfTheBlockGasLimit,
			"--value=10", fortest.TestAddresses[1].String(),
		)

		blk, err := n.BlockMining.BlockMiningAPI.MiningOnce(ctx)
		require.NoError(t, err)
		blkCid := blk.Cid().String()

		blockInfo := cmdClient.RunSuccess(ctx, "show", "header", blkCid, "--enc", "json").ReadStdoutTrimNewlines()

		require.NoError(t, json.Unmarshal([]byte(blockInfo), &result))
		assert.NotEmpty(t, result.Messages.SecpRoot, "msg under the block gas limit passes validation and is run in the block")
	})
}

func TestMessageStatus(t *testing.T) {
	tf.IntegrationTest(t)
	t.Skip("Dragons: fake proofs")

	ctx := context.Background()
	builder := test.NewNodeBuilder(t)

	buildWithMiner(t, builder)
	n, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	t.Run("queue then on chain", func(t *testing.T) {
		msg := cmdClient.RunSuccess(
			ctx,
			"message", "send",
			"--from", fortest.TestAddresses[0].String(),
			"--gas-price", "1", "--gas-limit", "300",
			"--value=1234",
			fortest.TestAddresses[1].String(),
		)

		msgcid := msg.ReadStdoutTrimNewlines()
		status := cmdClient.RunSuccess(ctx, "message", "status", msgcid).ReadStdout()

		assert.Contains(t, status, "In outbox")
		assert.Contains(t, status, "In mpool")
		assert.NotContains(t, status, "On chain") // not found on chain (yet)
		assert.Contains(t, status, "1234")        // the "value"

		_, err := n.BlockMining.BlockMiningAPI.MiningOnce(ctx)
		require.NoError(t, err)

		status = cmdClient.RunSuccess(ctx, "message", "status", msgcid).ReadStdout()

		assert.NotContains(t, status, "In outbox")
		assert.NotContains(t, status, "In mpool")
		assert.Contains(t, status, "On chain")
		assert.Contains(t, status, "1234") // the "value"

		status = cmdClient.RunSuccess(ctx, "message", "status", "QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS").ReadStdout()
		assert.NotContains(t, status, "In outbox")
		assert.NotContains(t, status, "In mpool")
		assert.NotContains(t, status, "On chain")
	})
}
