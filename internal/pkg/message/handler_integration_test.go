package message_test

import (
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
	"github.com/filecoin-project/go-filecoin/internal/pkg/journal"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/account"
)

// TestNewHeadHandlerIntegration tests inbox and outbox policy consistency.
func TestNewHeadHandlerIntegration(t *testing.T) {
	tf.UnitTest(t)
	signer, _ := types.NewMockSignersAndKeyInfo(2)
	objournal := journal.NewInMemoryJournal(t, th.NewFakeClock(time.Unix(1234567890, 0))).Topic("outbox")
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

	makeHandler := func(provider *message.FakeProvider, root block.TipSet) *message.HeadHandler {
		mpool := message.NewPool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		inbox := message.NewInbox(mpool, maxAge, provider, provider)
		queue := message.NewQueue()
		publisher := message.NewDefaultPublisher(&message.MockNetworkPublisher{}, mpool)
		policy := message.NewMessageQueuePolicy(provider, maxAge)
		outbox := message.NewOutbox(signer, &message.FakeValidator{}, queue, publisher, policy,
			provider, provider, objournal)

		return message.NewHeadHandler(inbox, outbox, provider, root)
	}

	t.Run("test send after reverted message", func(t *testing.T) {
		provider := message.NewFakeProvider(t)
		root := provider.NewGenesis()
		actr, _ := account.NewActor(types.ZeroAttoFIL)
		actr.Nonce = 42
		provider.SetHeadAndActor(t, root.Key(), sender, actr)

		handler := makeHandler(provider, root)
		outbox := handler.Outbox
		inbox := handler.Inbox

		// First, send a message and expect to find it in the message queue and pool.
		mid1, donePub1, err := outbox.Send(ctx, sender, dest, types.ZeroAttoFIL, gasPrice, gasUnits, true, types.MethodID(9000001))
		require.NoError(t, err)
		require.NotNil(t, donePub1)
		require.Equal(t, 1, len(outbox.Queue().List(sender))) // Message is in the queue.
		pub1Err := <-donePub1
		assert.NoError(t, pub1Err)
		msg1, found := inbox.Pool().Get(mid1)
		require.True(t, found) // Message is in the pool.
		assert.True(t, msg1.Equals(outbox.Queue().List(sender)[0].Msg))

		// Receive the message in a block.
		left := provider.BuildOneOn(root, func(b *chain.BlockBuilder) {
			b.AddMessages([]*types.SignedMessage{msg1}, []*types.UnsignedMessage{})
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
		assert.Equal(t, 1, len(outbox.Queue().List(sender))) // Message returns to queue.
		_, found = inbox.Pool().Get(mid1)
		assert.True(t, found) // Message returns to pool to be mined again.

		// Send another message from the same account.
		// First, send a message and expect to find it in the message queue and pool.
		mid2, donePub2, err := outbox.Send(ctx, sender, dest, types.ZeroAttoFIL, gasPrice, gasUnits, true, types.MethodID(9000002))
		// This case causes the nonce to be wrongly calculated, since the first, now-unmined message
		// is not in the outbox, and actor state has not updated, but the message pool already has
		// a message with the same nonce.
		require.NoError(t, err)
		require.NotNil(t, donePub2)

		// Both messages are in the pool.
		pub2Err := <-donePub2
		assert.NoError(t, pub2Err)
		_, found = inbox.Pool().Get(mid1)
		require.True(t, found)
		msg2, found := inbox.Pool().Get(mid2)
		require.True(t, found)
		// Both messages are in the queue too, in the right order.
		restoredQueue := outbox.Queue().List(sender)
		assert.Equal(t, 2, len(restoredQueue))
		assert.True(t, msg1.Equals(restoredQueue[0].Msg))
		assert.True(t, msg2.Equals(restoredQueue[1].Msg))
	})

	t.Run("ignores empty tipset", func(t *testing.T) {
		provider := message.NewFakeProvider(t)
		root := provider.NewGenesis()
		provider.SetHead(root.Key())

		handler := makeHandler(provider, root)
		err := handler.HandleNewHead(ctx, block.UndefTipSet)
		assert.NoError(t, err)
	})

	t.Run("ignores duplicate tipset", func(t *testing.T) {
		provider := message.NewFakeProvider(t)
		root := provider.NewGenesis()
		provider.SetHead(root.Key())

		handler := makeHandler(provider, root)
		err := handler.HandleNewHead(ctx, root)
		assert.NoError(t, err)
	})
}
