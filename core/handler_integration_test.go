package core_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/core"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

// TestNewHeadHandlerIntegration tests inbox and outbox policy consistency.
func TestNewHeadHandlerIntegration(t *testing.T) {
	tf.UnitTest(t)
	signer, _ := types.NewMockSignersAndKeyInfo(2)
	sender := signer.Addresses[0]
	dest := signer.Addresses[1]
	ctx := context.Background()
	// Maximum age for a message in the pool/queue. As of August 2019, this is in rounds for the
	// outbox queue and non-null tipsets for the inbox pool :-(.
	// We generally desire messages to expire from the pool before the queue so that retries are
	// accepted.
	maxAge := uint(10)
	gasPrice := types.NewGasPrice(1)
	gasUnits := types.NewGasUnits(1000)

	makeHandler := func(provider *core.FakeProvider, root types.TipSet) *core.NewHeadHandler {
		mpool := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		inbox := core.NewInbox(mpool, maxAge, provider, provider)
		queue := core.NewMessageQueue()
		publisher := core.NewDefaultMessagePublisher(&core.MockNetworkPublisher{}, "Topic", mpool)
		policy := core.NewMessageQueuePolicy(provider, maxAge)
		outbox := core.NewOutbox(signer, &core.FakeValidator{}, queue, publisher, policy, provider, provider)

		return core.NewHandler(inbox, outbox, provider, root)
	}

	t.Run("test send after reverted message", func(t *testing.T) {
		provider := core.NewFakeProvider(t)
		root := provider.NewGenesis()
		actr, _ := account.NewActor(types.ZeroAttoFIL)
		actr.Nonce = 42
		provider.SetHeadAndActor(t, root.Key(), sender, actr)

		handler := makeHandler(provider, root)
		outbox := handler.Outbox
		inbox := handler.Inbox

		// First, send a message and expect to find it in the message queue and pool.
		mid1, err := outbox.Send(ctx, sender, dest, types.ZeroAttoFIL, gasPrice, gasUnits, true, "method1")
		require.NoError(t, err)
		require.Equal(t, 1, len(outbox.Queue().List(sender))) // Message is in the queue.
		msg1, found := inbox.Pool().Get(mid1)
		require.True(t, found) // Message is in the pool.
		assert.True(t, msg1.Equals(outbox.Queue().List(sender)[0].Msg))

		// Receive the message in a block.
		left := provider.BuildOneOn(root, func(b *chain.BlockBuilder) {
			b.AddMessages([]*types.SignedMessage{msg1}, types.EmptyReceipts(1))
		})
		require.NoError(t, handler.HandleNewHead(ctx, left))
		assert.Equal(t, 0, len(outbox.Queue().List(sender))) // Gone from queue.
		_, found = inbox.Pool().Get(mid1)
		assert.False(t, found) // Gone from pool.

		// Now re-org the chain to un-mine that message.
		right := provider.BuildOneOn(root, func(b *chain.BlockBuilder) {
			// No messages.
		})
		require.NoError(t, handler.HandleNewHead(ctx, right))
		assert.Equal(t, 0, len(outbox.Queue().List(sender))) // Message does not return to queue.
		_, found = inbox.Pool().Get(mid1)
		assert.True(t, found) // Message returns to pool to be mined again.

		// Send another message from the same account.
		// First, send a message and expect to find it in the message queue and pool.
		_, err = outbox.Send(ctx, sender, dest, types.ZeroAttoFIL, gasPrice, gasUnits, true, "method2")
		// This case causes the nonce to be wrongly calculated, since the first, now-unmined message
		// is not in the outbox, and actor state has not updated, but the message pool already has
		// a message with the same nonce.
		assert.Error(t, err, "#3052 is fixed!")
		assert.Contains(t, err.Error(), "same actor and nonce")
		//require.NoError(t, err)
		//require.Equal(t, 1, len(queue.List(sender))) // The new message is in the queue.
		//// Both messages are in the pool.
		//msg2, found := mpool.Get(mid2)
		//require.True(t, found)
		//_, found = mpool.Get(mid1)
		//require.True(t, found)
		//assert.True(t, msg2.Equals(queue.List(sender)[0].Msg))
	})

	t.Run("ignores empty tipset", func(t *testing.T) {
		provider := core.NewFakeProvider(t)
		root := provider.NewGenesis()
		provider.SetHead(root.Key())

		handler := makeHandler(provider, root)
		err := handler.HandleNewHead(ctx, types.UndefTipSet)
		assert.NoError(t, err)
	})

	t.Run("ignores duplicate tipset", func(t *testing.T) {
		provider := core.NewFakeProvider(t)
		root := provider.NewGenesis()
		provider.SetHead(root.Key())

		handler := makeHandler(provider, root)
		err := handler.HandleNewHead(ctx, root)
		assert.NoError(t, err)
	})
}
