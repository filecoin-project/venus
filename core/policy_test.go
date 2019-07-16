package core_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/core"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

// Tests for the outbound message queue policy.
// These tests could use a fake/mock policy target, but it would require some sophistication to
// validate the order of removals, so using a real queue is a bit easier.
func TestMessageQueuePolicy(t *testing.T) {
	tf.UnitTest(t)

	// Individual tests share a MessageMaker so not parallel (but quick)
	ctx := context.Background()

	keys := types.MustGenerateKeyInfo(2, 42)
	mm := types.NewMessageMaker(t, keys)

	alice := mm.Addresses()[0]
	bob := mm.Addresses()[1]

	requireEnqueue := func(q *core.MessageQueue, msg *types.SignedMessage, stamp uint64) *types.SignedMessage {
		err := q.Enqueue(ctx, msg, stamp)
		require.NoError(t, err)
		return msg
	}

	t.Run("old block does nothing", func(t *testing.T) {
		blocks := chain.NewBuilder(t, alice)
		q := core.NewMessageQueue()
		policy := core.NewMessageQueuePolicy(blocks, 10)

		fromAlice := mm.NewSignedMessage(alice, 1)
		fromBob := mm.NewSignedMessage(bob, 1)
		requireEnqueue(q, fromAlice, 100)
		requireEnqueue(q, fromBob, 200)

		root := blocks.AppendOn() // Height = 0
		b1 := blocks.AppendOn(root)

		err := policy.HandleNewHead(ctx, q, requireTipset(t, root), requireTipset(t, b1))
		assert.NoError(t, err)
		assert.Equal(t, qm(fromAlice, 100), q.List(alice)[0])
		assert.Equal(t, qm(fromBob, 200), q.List(bob)[0])
	})

	t.Run("removes mined messages", func(t *testing.T) {
		blocks := chain.NewBuilder(t, alice)
		q := core.NewMessageQueue()
		policy := core.NewMessageQueuePolicy(blocks, 10)

		msgs := []*types.SignedMessage{
			requireEnqueue(q, mm.NewSignedMessage(alice, 1), 100),
			requireEnqueue(q, mm.NewSignedMessage(alice, 2), 101),
			requireEnqueue(q, mm.NewSignedMessage(alice, 3), 102),
			requireEnqueue(q, mm.NewSignedMessage(bob, 1), 100),
		}

		assert.Equal(t, qm(msgs[0], 100), q.List(alice)[0])
		assert.Equal(t, qm(msgs[3], 100), q.List(bob)[0])

		root := blocks.BuildOn(nil, func(b *chain.BlockBuilder) {
			b.IncHeight(103)
		})
		b1 := blocks.BuildOn(root, func(b *chain.BlockBuilder) {
			b.AddMessage(msgs[0], &types.MessageReceipt{})
		})

		err := policy.HandleNewHead(ctx, q, requireTipset(t, root), requireTipset(t, b1))
		require.NoError(t, err)
		assert.Equal(t, qm(msgs[1], 101), q.List(alice)[0]) // First message removed successfully
		assert.Equal(t, qm(msgs[3], 100), q.List(bob)[0])   // No change

		// A block with no messages does nothing
		b2 := blocks.AppendOn(b1)
		err = policy.HandleNewHead(ctx, q, requireTipset(t, b1), requireTipset(t, b2))
		require.NoError(t, err)
		assert.Equal(t, qm(msgs[1], 101), q.List(alice)[0])
		assert.Equal(t, qm(msgs[3], 100), q.List(bob)[0])

		// Block with both alice and bob's next message
		b3 := blocks.BuildOn(b2, func(b *chain.BlockBuilder) {
			b.AddMessage(msgs[1], &types.MessageReceipt{})
			b.AddMessage(msgs[3], &types.MessageReceipt{})
		})
		err = policy.HandleNewHead(ctx, q, requireTipset(t, b2), requireTipset(t, b3))
		require.NoError(t, err)
		assert.Equal(t, qm(msgs[2], 102), q.List(alice)[0])
		assert.Empty(t, q.List(bob)) // None left

		// Block with alice's last message
		b4 := blocks.BuildOn(b3, func(b *chain.BlockBuilder) {
			b.AddMessage(msgs[2], &types.MessageReceipt{})
		})
		err = policy.HandleNewHead(ctx, q, requireTipset(t, b3), requireTipset(t, b4))
		require.NoError(t, err)
		assert.Empty(t, q.List(alice))
	})

	t.Run("expires old messages", func(t *testing.T) {
		blocks := chain.NewBuilder(t, alice)
		q := core.NewMessageQueue()
		policy := core.NewMessageQueuePolicy(blocks, 10)

		msgs := []*types.SignedMessage{
			requireEnqueue(q, mm.NewSignedMessage(alice, 1), 100),
			requireEnqueue(q, mm.NewSignedMessage(alice, 2), 101),
			requireEnqueue(q, mm.NewSignedMessage(alice, 3), 102),
			requireEnqueue(q, mm.NewSignedMessage(bob, 1), 200),
		}

		assert.Equal(t, qm(msgs[0], 100), q.List(alice)[0])
		assert.Equal(t, qm(msgs[3], 200), q.List(bob)[0])

		root := blocks.BuildOn(nil, func(b *chain.BlockBuilder) {
			b.IncHeight(100)
		})

		// Skip 9 rounds since alice's first message enqueued, so b1 has height 110
		b1 := blocks.BuildOn(root, func(b *chain.BlockBuilder) {
			b.IncHeight(9)
		})

		err := policy.HandleNewHead(ctx, q, requireTipset(t, root), requireTipset(t, b1))
		require.NoError(t, err)

		assert.Equal(t, qm(msgs[0], 100), q.List(alice)[0]) // No change
		assert.Equal(t, qm(msgs[3], 200), q.List(bob)[0])

		b2 := blocks.AppendOn(b1) // Height b1.Height + 1 = 111
		err = policy.HandleNewHead(ctx, q, requireTipset(t, b1), requireTipset(t, b2))
		require.NoError(t, err)
		assert.Empty(t, q.List(alice))                    // Alice's messages all expired
		assert.Equal(t, qm(msgs[3], 200), q.List(bob)[0]) // Bob's remain
	})

	t.Run("fails when messages out of nonce order", func(t *testing.T) {
		blocks := chain.NewBuilder(t, alice)
		q := core.NewMessageQueue()
		policy := core.NewMessageQueuePolicy(blocks, 10)

		msgs := []*types.SignedMessage{
			requireEnqueue(q, mm.NewSignedMessage(alice, 1), 100),
			requireEnqueue(q, mm.NewSignedMessage(alice, 2), 101),
			requireEnqueue(q, mm.NewSignedMessage(alice, 3), 102),
		}

		root := blocks.BuildOn(nil, func(b *chain.BlockBuilder) {
			b.IncHeight(100)
		})

		b1 := blocks.BuildOn(root, func(b *chain.BlockBuilder) {
			b.AddMessage(msgs[1], &types.MessageReceipt{})
		})
		err := policy.HandleNewHead(ctx, q, requireTipset(t, root), requireTipset(t, b1))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nonce 1, expected 2")
	})

	t.Run("removes sequential messages in peer blocks", func(t *testing.T) {
		blocks := chain.NewBuilder(t, alice)
		q := core.NewMessageQueue()
		policy := core.NewMessageQueuePolicy(blocks, 10)

		msgs := []*types.SignedMessage{
			requireEnqueue(q, mm.NewSignedMessage(alice, 1), 100),
			requireEnqueue(q, mm.NewSignedMessage(alice, 2), 101),
		}

		root := blocks.BuildOn(nil, func(b *chain.BlockBuilder) {
			b.IncHeight(100)
		})

		// Construct two blocks at the same height, each with one message. The canonical
		// tipset block ordering is given by block ticket, which matches this order.
		// These blocks are constructed so that their CIDs would order them
		// in the *opposite* order (blocks used to be ordered by CID).
		b1 := blocks.BuildOn(root, func(b *chain.BlockBuilder) {
			b.AddMessage(msgs[0], &types.MessageReceipt{})
			b.SetTicket([]byte{0})
		})
		b2 := blocks.BuildOn(root, func(b *chain.BlockBuilder) {
			b.AddMessage(msgs[1], &types.MessageReceipt{})
			b.SetTicket([]byte{1})
			b.SetTimestamp(1) // Tweak if necessary to force CID ordering opposite ticket ordering.
		})
		assert.True(t, bytes.Compare(b1.Cid().Bytes(), b2.Cid().Bytes()) > 0)

		// With blocks ordered [b1, b2], everything is ok.
		err := policy.HandleNewHead(ctx, q, requireTipset(t, root), requireTipset(t, b1, b2))
		require.NoError(t, err)
		assert.Empty(t, q.List(alice))

		// With blocks ordered [b2, b1], this fails. This demonstrates that the policy is
		// processing the blocks in canonical (ticket) order.
		requireEnqueue(q, msgs[0], 200)
		requireEnqueue(q, msgs[1], 201)
		b1.Ticket = []byte{1}
		b2.Ticket = []byte{0}
		err = policy.HandleNewHead(ctx, q, requireTipset(t, root), requireTipset(t, b1, b2))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nonce 1, expected 2")
	})
}

func requireTipset(t *testing.T, blocks ...*types.Block) types.TipSet {
	set, err := types.NewTipSet(blocks...)
	require.NoError(t, err)
	return set
}

func qm(msg *types.SignedMessage, stamp uint64) *core.QueuedMessage {
	return &core.QueuedMessage{Msg: msg, Stamp: stamp}
}
