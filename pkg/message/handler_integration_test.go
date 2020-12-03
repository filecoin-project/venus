package message_test

import (
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/clock"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/journal"
	"github.com/filecoin-project/venus/pkg/message"
	th "github.com/filecoin-project/venus/pkg/testhelpers"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/pkg/types"
)

// TestNewHeadHandlerIntegration tests inbox and outbox policy consistency.
func TestNewHeadHandlerIntegration(t *testing.T) {
	tf.UnitTest(t)
	signer, _ := types.NewMockSignersAndKeyInfo(2)
	objournal := journal.NewInMemoryJournal(t, clock.NewFake(time.Unix(1234567890, 0))).Topic("outbox")
	sender := signer.Addresses[0]
	dest := signer.Addresses[1]

	// Interleave 1 signers
	kis := types.MustGenerateBLSKeyInfo(2, 0)
	signer2 := types.NewMockSigner(kis)
	sender2 := signer2.Addresses[0]
	dest2 := signer2.Addresses[1]

	ctx := context.Background()
	// Maximum age for a message in the pool/queue. As of August 2019, this is in rounds for the
	// outbox queue and non-null tipsets for the inbox pool :-(.
	// We generally desire messages to expire from the pool before the queue so that retries are
	// accepted.
	maxAge := uint(10)
	gasPrice := types.NewGasPremium(1)
	gasPremium := types.NewGasPremium(1)
	gasUnits := types.NewGas(1000)
	gasFeeCap := types.NewGasFeeCap(1000)

	makeHandler := func(provider *message.FakeProvider, root *block.TipSet, signer types.Signer) *message.HeadHandler {
		mpool := message.NewPool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		inbox := message.NewInbox(mpool, maxAge, provider, provider)
		queue := message.NewQueue()
		publisher := message.NewDefaultPublisher(&message.MockNetworkPublisher{}, mpool)
		policy := message.NewMessageQueuePolicy(provider, maxAge)
		gp := message.NewGasPredictor("gasPredictor")
		outbox := message.NewOutbox(signer, &message.FakeValidator{}, queue, publisher, policy,
			provider, provider, objournal, gp)

		return message.NewHeadHandler(inbox, outbox, provider)
	}

	t.Run("test send after reverted message", func(t *testing.T) {
		provider := message.NewFakeProvider(t)
		root := provider.Genesis()
		actr := types.NewActor(builtin.AccountActorCodeID, abi.NewTokenAmount(0), cid.Undef)
		actr.Nonce = 42
		provider.SetHeadAndActor(t, root.Key(), sender, actr)

		handler := makeHandler(provider, root, signer)
		outbox := handler.Outbox
		inbox := handler.Inbox

		// First, send a message and expect to find it in the message queue and pool.
		mid1, donePub1, err := outbox.Send(ctx, sender, dest, types.ZeroAttoFIL, gasPrice, gasPremium, gasUnits, true, abi.MethodNum(9000001), []byte{})
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
		require.NoError(t, handler.HandleNewHead(ctx, nil, []*block.TipSet{left}))
		assert.Equal(t, 0, len(outbox.Queue().List(sender))) // Gone from queue.
		_, found = inbox.Pool().Get(mid1)
		assert.False(t, found) // Gone from pool.

		// Now re-org the chain to un-mine that message.
		right := provider.BuildOneOn(root, func(b *chain.BlockBuilder) {
			// No messages.
		})
		require.NoError(t, handler.HandleNewHead(ctx, []*block.TipSet{left}, []*block.TipSet{right}))
		assert.Equal(t, 1, len(outbox.Queue().List(sender))) // Message returns to queue.
		_, found = inbox.Pool().Get(mid1)
		assert.True(t, found) // Message returns to pool to be mined again.

		// Send another message from the same account.
		// First, send a message and expect to find it in the message queue and pool.
		mid2, donePub2, err := outbox.Send(ctx, sender, dest, types.ZeroAttoFIL, gasPrice, gasPremium, gasUnits, true, abi.MethodNum(9000002), []byte{})
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

	t.Run("test send after reverted bls message", func(t *testing.T) {
		provider := message.NewFakeProvider(t)
		root := provider.Genesis()
		actor := types.NewActor(builtin.AccountActorCodeID, abi.NewTokenAmount(0), cid.Undef)
		actor.Nonce = 42
		provider.SetHeadAndActor(t, root.Key(), sender2, actor)

		handler := makeHandler(provider, root, signer2)
		outbox := handler.Outbox
		inbox := handler.Inbox

		msg := types.NewUnsignedMessage(sender2, dest2, 0, types.ZeroAttoFIL, 0, nil)
		msg.GasPremium = gasPremium
		msg.GasLimit = gasUnits
		msg.GasFeeCap = gasFeeCap

		// First, send a message and expect to find it in the message queue and pool.
		mid1, donePub1, err := outbox.UnSignedSend(ctx, *msg)
		require.NoError(t, err)
		require.NotNil(t, donePub1)
		require.Equal(t, 1, len(outbox.Queue().List(sender2))) // Message is in the queue.
		pub1Err := <-donePub1
		assert.NoError(t, pub1Err)

		msg1, found := inbox.Pool().Get(mid1)
		require.True(t, found) // Message is in the pool.
		assert.True(t, msg1.Equals(outbox.Queue().List(sender2)[0].Msg))

		// Receive the message in a block.
		left := provider.BuildOneOn(root, func(b *chain.BlockBuilder) {
			b.AddMessages([]*types.SignedMessage{}, []*types.UnsignedMessage{&msg1.Message})
		})
		require.NoError(t, handler.HandleNewHead(ctx, nil, []*block.TipSet{left}))
		assert.Equal(t, 0, len(outbox.Queue().List(sender2))) // Gone from queue.
		_, found = inbox.Pool().Get(mid1)
		assert.False(t, found) // Gone from pool.

		// Now re-org the chain to un-mine that message.
		right := provider.BuildOneOn(root, func(b *chain.BlockBuilder) {
			// No messages.
		})
		require.NoError(t, handler.HandleNewHead(ctx, nil, []*block.TipSet{right}))
		assert.Equal(t, 0, len(outbox.Queue().List(sender2))) // Message returns to queue.
		_, found = inbox.Pool().Get(mid1)
		assert.False(t, found) // Message not returns to pool to be mined again, since BLS message is missing its signature

		// Send another message from the same account.
		// First, send a message and expect to find it in the message queue and pool.
		mid2, donePub2, err := outbox.Send(ctx, sender2, dest2, types.ZeroAttoFIL, gasPrice, gasPremium, gasUnits, true, abi.MethodNum(9000002), []byte{})
		// This case causes the nonce to be wrongly calculated, since the first, now-unmined message
		// is not in the outbox, and actor state has not updated, but the message pool already has
		// a message with the same nonce.
		require.NoError(t, err)
		require.NotNil(t, donePub2)

		// messages(`msg2`) in the pool.
		pub2Err := <-donePub2
		assert.NoError(t, pub2Err)
		_, found = inbox.Pool().Get(mid1)
		require.False(t, found)
		msg2, found := inbox.Pool().Get(mid2)
		require.True(t, found)
		// messages(`msg2`) is in the queue.
		restoredQueue := outbox.Queue().List(sender2)
		assert.Equal(t, 1, len(restoredQueue))
		assert.True(t, msg2.Equals(restoredQueue[0].Msg))
	})

	t.Run("ignores empty tipset", func(t *testing.T) {
		provider := message.NewFakeProvider(t)
		root := provider.Genesis()
		provider.SetHead(root.Key())

		handler := makeHandler(provider, root, signer)
		err := handler.HandleNewHead(ctx, nil, nil)
		assert.NoError(t, err)
	})

	t.Run("ignores duplicate tipset", func(t *testing.T) {
		provider := message.NewFakeProvider(t)
		root := provider.Genesis()
		provider.SetHead(root.Key())

		handler := makeHandler(provider, root, signer)
		err := handler.HandleNewHead(ctx, nil, []*block.TipSet{root})
		assert.NoError(t, err)
	})
}
