package commands_test

import (
	"context"
	"sync"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/fixtures/fortest"
	"github.com/filecoin-project/venus/internal/app/go-filecoin/node"
	"github.com/filecoin-project/venus/internal/app/go-filecoin/node/test"
	th "github.com/filecoin-project/venus/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
)

func TestMpoolLs(t *testing.T) {
	tf.IntegrationTest(t)
	t.Skip("hangs")

	sendMessage := func(ctx context.Context, cmdClient *test.Client, from address.Address, to address.Address) *th.CmdOutput {
		return cmdClient.RunSuccess(ctx, "message", "send",
			"--from", from.String(),
			"--gas-price", "1", "--gas-limit", "300",
			"--value=10", to.String(),
		)
	}
	cs := node.FixtureChainSeed(t)

	t.Run("return all messages", func(t *testing.T) {
		ctx := context.Background()
		builder := test.NewNodeBuilder(t)
		builder.WithInitOpt(cs.KeyInitOpt(0))
		builder.WithGenesisInit(cs.GenesisInitFunc)

		_, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		sendMessage(ctx, cmdClient, fortest.TestAddresses[0], fortest.TestAddresses[2])
		sendMessage(ctx, cmdClient, fortest.TestAddresses[0], fortest.TestAddresses[2])

		cids := cmdClient.RunSuccessLines(ctx, "mpool", "ls")

		assert.Equal(t, 2, len(cids))

		for _, c := range cids {
			ci, err := cid.Decode(c)
			assert.NoError(t, err)
			assert.True(t, ci.Defined())
		}

		// Should return immediately with --wait-for-count equal to message count
		cids = cmdClient.RunSuccessLines(ctx, "mpool", "ls", "--wait-for-count=2")
		assert.Equal(t, 2, len(cids))
	})

	t.Run("wait for enough messages", func(t *testing.T) {
		ctx := context.Background()
		builder := test.NewNodeBuilder(t)
		builder.WithInitOpt(cs.KeyInitOpt(0))
		builder.WithGenesisInit(cs.GenesisInitFunc)

		_, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		wg := sync.WaitGroup{}
		wg.Add(1)

		complete := false
		go func() {
			c := cmdClient.RunSuccessLines(ctx, "mpool", "ls", "--wait-for-count=3")
			complete = true
			assert.Equal(t, 3, len(c))
			wg.Done()
		}()

		sendMessage(ctx, cmdClient, fortest.TestAddresses[0], fortest.TestAddresses[1])
		assert.False(t, complete)
		sendMessage(ctx, cmdClient, fortest.TestAddresses[0], fortest.TestAddresses[1])
		assert.False(t, complete)
		sendMessage(ctx, cmdClient, fortest.TestAddresses[0], fortest.TestAddresses[1])

		wg.Wait()

		assert.True(t, complete)
	})
}

func TestMpoolShow(t *testing.T) {
	tf.IntegrationTest(t)
	t.Skip("hangs")
	cs := node.FixtureChainSeed(t)

	t.Run("shows message", func(t *testing.T) {

		ctx := context.Background()
		builder := test.NewNodeBuilder(t)
		builder.WithInitOpt(cs.KeyInitOpt(0))
		builder.WithGenesisInit(cs.GenesisInitFunc)

		_, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		msgCid := cmdClient.RunSuccess(ctx, "message", "send",
			"--from", fortest.TestAddresses[0].String(),
			"--gas-price", "1", "--gas-limit", "300",
			"--value=10", fortest.TestAddresses[2].String(),
		).ReadStdoutTrimNewlines()

		out := cmdClient.RunSuccess(ctx, "mpool", "show", msgCid).ReadStdoutTrimNewlines()

		assert.Contains(t, out, "From:      "+fortest.TestAddresses[0].String())
		assert.Contains(t, out, "To:        "+fortest.TestAddresses[2].String())
		assert.Contains(t, out, "Value:     10")
	})

	t.Run("fails missing message", func(t *testing.T) {

		ctx := context.Background()
		builder := test.NewNodeBuilder(t)
		builder.WithInitOpt(cs.KeyInitOpt(0))
		builder.WithGenesisInit(cs.GenesisInitFunc)

		_, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		const c = "QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw"

		out := cmdClient.RunFail(ctx, "not found", "mpool", "show", c).ReadStderr()
		assert.Contains(t, out, c)
	})
}

func TestMpoolRm(t *testing.T) {
	tf.IntegrationTest(t)
	t.Skip("hangs")

	t.Run("remove a message", func(t *testing.T) {
		cs := node.FixtureChainSeed(t)
		ctx := context.Background()
		builder := test.NewNodeBuilder(t)
		builder.WithInitOpt(cs.KeyInitOpt(0))
		builder.WithGenesisInit(cs.GenesisInitFunc)

		n, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		msgCid := cmdClient.RunSuccess(ctx, "message", "send",
			"--from", fortest.TestAddresses[0].String(),
			"--gas-price", "1", "--gas-limit", "300",
			"--value=10", fortest.TestAddresses[2].String(),
		).ReadStdoutTrimNewlines()

		// wait for the pool to have the message
		_, err := n.PorcelainAPI.MessagePoolWait(ctx, 1)
		require.NoError(t, err)

		// remove message in process so the following ls cannot race on lock
		//  acquire
		c, err := cid.Parse(msgCid)
		require.NoError(t, err)
		n.PorcelainAPI.MessagePoolRemove(c)

		out := cmdClient.RunSuccess(ctx, "mpool", "ls").ReadStdoutTrimNewlines()

		assert.Equal(t, "", out)
	})
}
