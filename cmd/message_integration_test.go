package cmd_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/filecoin-project/venus/pkg/constants"

	"github.com/filecoin-project/go-address"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/app/node/test"
	"github.com/filecoin-project/venus/fixtures/fortest"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func TestMessageSend(t *testing.T) {
	t.Skip("This can be unskipped with fake proofs")
	tf.IntegrationTest(t)
	ctx := context.Background()
	builder := test.NewNodeBuilder(t)
	defaultAddr := fortest.TestAddresses[0]

	cs := test.FixtureChainSeed(t)
	builder.WithGenesisInit(cs.GenesisInitFunc)
	builder.WithConfig(test.DefaultAddressConfigOpt(defaultAddr))
	builder.WithInitOpt(cs.KeyInitOpt(1))
	builder.WithInitOpt(cs.KeyInitOpt(0))

	n, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	from, err := n.Wallet().API().WalletDefaultAddress(ctx) // this should = fixtures.TestAddresses[0]
	require.NoError(t, err)

	t.Log("[failure] invalid target")
	cmdClient.RunFail(
		ctx,
		address.ErrUnknownNetwork.Error(),
		"send",
		"--from", from.String(),
		"--gas-price", "0", "--gas-limit", "300",
		"--value=10", "xyz",
	)

	t.Log("[success] with from")
	cmdClient.RunSuccess(
		ctx,
		"send",
		"--from", from.String(),
		"--gas-price", "1",
		"--gas-limit", "300",
		fortest.TestAddresses[3].String(),
	)

	t.Log("[success] with from and int value")
	cmdClient.RunSuccess(
		ctx,
		"send",
		"--from", from.String(),
		"--gas-price", "1",
		"--gas-limit", "300",
		"--value", "10",
		fortest.TestAddresses[3].String(),
	)

	t.Log("[success] with from and decimal value")
	cmdClient.RunSuccess(
		ctx,
		"send",
		"--from", from.String(),
		"--gas-price", "1",
		"--gas-limit", "300",
		"--value", "5.5",
		fortest.TestAddresses[3].String(),
	)
}

func TestMessageSendBlockGasLimit(t *testing.T) {
	tf.IntegrationTest(t)
	t.Skip("Unskip using fake proofs")

	ctx := context.Background()
	builder := test.NewNodeBuilder(t)
	defaultAddr := fortest.TestAddresses[0]

	buildWithMiner(t, builder)
	builder.WithConfig(test.DefaultAddressConfigOpt(defaultAddr))
	_, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	doubleTheBlockGasLimit := strconv.Itoa(int(constants.BlockGasLimit) * 2)

	t.Run("when the gas limit is above the block limit, the message fails", func(t *testing.T) {
		cmdClient.RunFail(
			ctx,
			"block gas limit",
			"send",
			"--gas-price", "1", "--gas-limit", doubleTheBlockGasLimit,
			"--value=10", fortest.TestAddresses[1].String(),
		)
	})
}
