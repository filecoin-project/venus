package commands_test

import (
	"context"
	"github.com/filecoin-project/venus/internal/pkg/constants"
	"strconv"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commands "github.com/filecoin-project/venus/cmd/go-filecoin"
	"github.com/filecoin-project/venus/fixtures/fortest"
	"github.com/filecoin-project/venus/internal/app/go-filecoin/node"
	"github.com/filecoin-project/venus/internal/app/go-filecoin/node/test"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
)

func TestMessageSend(t *testing.T) {
	t.Skip("This can be unskipped with fake proofs")
	tf.IntegrationTest(t)
	ctx := context.Background()
	builder := test.NewNodeBuilder(t)
	defaultAddr := fortest.TestAddresses[0]

	cs := node.FixtureChainSeed(t)
	builder.WithGenesisInit(cs.GenesisInitFunc)
	builder.WithConfig(node.DefaultAddressConfigOpt(defaultAddr))
	builder.WithInitOpt(cs.KeyInitOpt(1))
	builder.WithInitOpt(cs.KeyInitOpt(0))

	n, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

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
	tf.IntegrationTest(t)
	ctx := context.Background()

	seed, genCfg, chainClock := test.CreateBootstrapSetup(t)
	node := test.CreateBootstrapMiner(ctx, t, seed, chainClock, genCfg)

	cmdClient, clientStop := test.RunNodeAPI(ctx, node, t)
	defer clientStop()

	t.Run("[success] transfer only", func(t *testing.T) {
		var sendResult commands.MessageSendResult
		cmdClient.RunMarshaledJSON(ctx, &sendResult, "message", "send",
			"--gas-price", "1",
			"--gas-limit", "300",
			fortest.TestAddresses[1].String(),
		)

		// Fail with timeout before the message has been mined
		cmdClient.RunFail(
			ctx,
			"deadline exceeded",
			"message", "wait",
			"--message=false",
			"--receipt=false",
			"--timeout=100ms",
			"--return",
			sendResult.Cid.String(),
		)

		var waitResult commands.WaitResult
		cmdClient.RunMarshaledJSON(
			ctx,
			&waitResult,
			"message", "wait",
			"--message=false",
			"--receipt=false",
			"--timeout=1m",
			"--return",
			sendResult.Cid.String(),
		)
		assert.Equal(t, fortest.TestAddresses[1], waitResult.Message.Message.To)
	})

	t.Run("[success] lookback", func(t *testing.T) {
		var sendResult commands.MessageSendResult
		cmdClient.RunMarshaledJSON(ctx, &sendResult, "message", "send",
			"--from", fortest.TestAddresses[0].String(),
			"--gas-price", "1",
			"--gas-limit", "300",
			fortest.TestAddresses[1].String(),
		)

		// Fail with timeout because the message is too early for the default lookback (2)
		cmdClient.RunFail(
			ctx,
			"deadline exceeded",
			"message", "wait",
			"--message=false",
			"--receipt=false",
			"--timeout=1s",
			"--return",
			sendResult.Cid.String(),
		)

		// succeed by specifying a higher lookback
		var waitResult commands.WaitResult
		cmdClient.RunMarshaledJSON(
			ctx,
			&waitResult,
			"message", "wait",
			"--message=false",
			"--receipt=false",
			"--lookback=10",
			"--timeout=1m",
			"--return",
			sendResult.Cid.String(),
		)
	})
}

func TestMessageSendBlockGasLimit(t *testing.T) {
	tf.IntegrationTest(t)
	t.Skip("Unskip using fake proofs")

	ctx := context.Background()
	builder := test.NewNodeBuilder(t)
	defaultAddr := fortest.TestAddresses[0]

	buildWithMiner(t, builder)
	builder.WithConfig(node.DefaultAddressConfigOpt(defaultAddr))
	_, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	doubleTheBlockGasLimit := strconv.Itoa(int(constants.BlockGasLimit) * 2)

	t.Run("when the gas limit is above the block limit, the message fails", func(t *testing.T) {
		cmdClient.RunFail(
			ctx,
			"block gas limit",
			"message", "send",
			"--gas-price", "1", "--gas-limit", doubleTheBlockGasLimit,
			"--value=10", fortest.TestAddresses[1].String(),
		)
	})
}
