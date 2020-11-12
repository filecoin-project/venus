package commands_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/venus/fixtures/fortest"
	"github.com/filecoin-project/venus/internal/app/go-filecoin/node"
	"github.com/filecoin-project/venus/internal/app/go-filecoin/node/test"
	th "github.com/filecoin-project/venus/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
)

func TestOutbox(t *testing.T) {
	tf.IntegrationTest(t)
	t.Skip("not working")

	sendMessage := func(ctx context.Context, cmdClient *test.Client, from address.Address, to address.Address) *th.CmdOutput {
		return cmdClient.RunSuccess(ctx, "message", "send",
			"--from", from.String(),
			"--gas-price", "1", "--gas-limit", "300",
			"--value=10", to.String(),
		)
	}
	cs := node.FixtureChainSeed(t)

	t.Run("list queued messages", func(t *testing.T) {
		ctx := context.Background()
		builder := test.NewNodeBuilder(t)
		builder.WithInitOpt(cs.KeyInitOpt(0))
		builder.WithInitOpt(cs.KeyInitOpt(1))
		builder.WithGenesisInit(cs.GenesisInitFunc)

		_, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		c1 := sendMessage(ctx, cmdClient, fortest.TestAddresses[0], fortest.TestAddresses[2]).ReadStdoutTrimNewlines()
		c2 := sendMessage(ctx, cmdClient, fortest.TestAddresses[0], fortest.TestAddresses[2]).ReadStdoutTrimNewlines()
		c3 := sendMessage(ctx, cmdClient, fortest.TestAddresses[1], fortest.TestAddresses[2]).ReadStdoutTrimNewlines()

		out := cmdClient.RunSuccess(ctx, "outbox", "ls").ReadStdout()
		assert.Contains(t, out, fortest.TestAddresses[0])
		assert.Contains(t, out, fortest.TestAddresses[1])
		assert.Contains(t, out, c1)
		assert.Contains(t, out, c2)
		assert.Contains(t, out, c3)

		// With address filter
		out = cmdClient.RunSuccess(ctx, "outbox", "ls", fortest.TestAddresses[1].String()).ReadStdout()
		assert.NotContains(t, out, fortest.TestAddresses[0])
		assert.Contains(t, out, fortest.TestAddresses[1])
		assert.NotContains(t, out, c1)
		assert.NotContains(t, out, c2)
		assert.Contains(t, out, c3)
	})

	t.Run("clear queue", func(t *testing.T) {
		ctx := context.Background()
		builder := test.NewNodeBuilder(t)
		builder.WithInitOpt(cs.KeyInitOpt(0))
		builder.WithInitOpt(cs.KeyInitOpt(1))
		builder.WithGenesisInit(cs.GenesisInitFunc)

		_, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		c1 := sendMessage(ctx, cmdClient, fortest.TestAddresses[0], fortest.TestAddresses[2]).ReadStdoutTrimNewlines()
		c2 := sendMessage(ctx, cmdClient, fortest.TestAddresses[0], fortest.TestAddresses[2]).ReadStdoutTrimNewlines()
		c3 := sendMessage(ctx, cmdClient, fortest.TestAddresses[1], fortest.TestAddresses[2]).ReadStdoutTrimNewlines()

		// With address filter
		cmdClient.RunSuccess(ctx, "outbox", "clear", fortest.TestAddresses[1].String())
		out := cmdClient.RunSuccess(ctx, "outbox", "ls").ReadStdout()
		assert.Contains(t, out, fortest.TestAddresses[0])
		assert.NotContains(t, out, fortest.TestAddresses[1]) // Cleared
		assert.Contains(t, out, c1)
		assert.Contains(t, out, c2)
		assert.NotContains(t, out, c3) // cleared

		// Repopulate
		sendMessage(ctx, cmdClient, fortest.TestAddresses[1], fortest.TestAddresses[2]).ReadStdoutTrimNewlines()

		// #nofilter
		cmdClient.RunSuccess(ctx, "outbox", "clear")
		out = cmdClient.RunSuccess(ctx, "outbox", "ls").ReadStdoutTrimNewlines()
		assert.Empty(t, out)

		// Clearing empty queue
		cmdClient.RunSuccess(ctx, "outbox", "clear")
		out = cmdClient.RunSuccess(ctx, "outbox", "ls").ReadStdoutTrimNewlines()
		assert.Empty(t, out)
	})
}
