package core_test

import (
	"context"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
	"testing"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
)

// Tests for the outbound message queue policy.
// These tests could use a fake/mock policy target, but it would require some sophistication to
// validate the order of removals, so using a real queue is a bit easier.
func TestMessageQueuePolicy(t *testing.T) {
	t.Parallel() // Individual tests share a MessageMaker so not parallel (but quick)
	ctx := context.Background()
	assert := assert.New(t)
	require := require.New(t)

	keys := types.MustGenerateKeyInfo(2, types.GenerateKeyInfoSeed())
	mm := types.NewMessageMaker(t, keys)

	alice := mm.Addresses()[0]
	bob := mm.Addresses()[1]

	requireEnqueue := func(q *core.MessageQueue, msg *types.SignedMessage, stamp uint64) *types.SignedMessage {
		err := q.Enqueue(msg, stamp)
		require.NoError(err)
		return msg
	}

	t.Run("old block does nothing", func(t *testing.T) {
		blocks := chain.NewFakeBlockProvider()
		q := core.NewMessageQueue()
		policy := core.NewMessageQueuePolicy(q, blocks, 10)

		fromAlice := mm.NewSignedMessage(alice, 1)
		fromBob := mm.NewSignedMessage(bob, 1)
		requireEnqueue(q, fromAlice, 100)
		requireEnqueue(q, fromBob, 200)

		root := blocks.NewBlock(0) // Height = 0
		b1 := blocks.NewBlockWithMessages(1, []*types.SignedMessage{}, root)

		err := policy.OnNewHeadTipset(ctx, requireTipset(t, root), requireTipset(t, b1))
		assert.NoError(err)
		assert.Equal(qm(fromAlice, 100), q.List(alice)[0])
		assert.Equal(qm(fromBob, 200), q.List(bob)[0])
	})

	t.Run("removes mined messages", func(t *testing.T) {
		blocks := chain.NewFakeBlockProvider()
		q := core.NewMessageQueue()
		policy := core.NewMessageQueuePolicy(q, blocks, 10)

		msgs := []*types.SignedMessage{
			requireEnqueue(q, mm.NewSignedMessage(alice, 1), 100),
			requireEnqueue(q, mm.NewSignedMessage(alice, 2), 101),
			requireEnqueue(q, mm.NewSignedMessage(alice, 3), 102),
			requireEnqueue(q, mm.NewSignedMessage(bob, 1), 100),
		}

		assert.Equal(qm(msgs[0], 100), q.List(alice)[0])
		assert.Equal(qm(msgs[3], 100), q.List(bob)[0])

		root := blocks.NewBlock(0)
		root.Height = 103
		b1 := blocks.NewBlockWithMessages(1, []*types.SignedMessage{msgs[0]}, root)

		err := policy.OnNewHeadTipset(ctx, requireTipset(t, root), requireTipset(t, b1))
		require.NoError(err)
		assert.Equal(qm(msgs[1], 101), q.List(alice)[0]) // First message removed successfully
		assert.Equal(qm(msgs[3], 100), q.List(bob)[0])   // No change

		// A block with no messages does nothing
		b2 := blocks.NewBlockWithMessages(2, []*types.SignedMessage{}, b1)
		err = policy.OnNewHeadTipset(ctx, requireTipset(t, b1), requireTipset(t, b2))
		require.NoError(err)
		assert.Equal(qm(msgs[1], 101), q.List(alice)[0])
		assert.Equal(qm(msgs[3], 100), q.List(bob)[0])

		// Block with both alice and bob's next message
		b3 := blocks.NewBlockWithMessages(3, []*types.SignedMessage{msgs[1], msgs[3]}, b2)
		err = policy.OnNewHeadTipset(ctx, requireTipset(t, b2), requireTipset(t, b3))
		require.NoError(err)
		assert.Equal(qm(msgs[2], 102), q.List(alice)[0])
		assert.Empty(q.List(bob)) // None left

		// Block with alice's last message
		b4 := blocks.NewBlockWithMessages(4, []*types.SignedMessage{msgs[2]}, b3)
		err = policy.OnNewHeadTipset(ctx, requireTipset(t, b3), requireTipset(t, b4))
		require.NoError(err)
		assert.Empty(q.List(alice))
	})

	t.Run("expires old messages", func(t *testing.T) {
		blocks := chain.NewFakeBlockProvider()
		q := core.NewMessageQueue()
		policy := core.NewMessageQueuePolicy(q, blocks, 10)

		msgs := []*types.SignedMessage{
			requireEnqueue(q, mm.NewSignedMessage(alice, 1), 100),
			requireEnqueue(q, mm.NewSignedMessage(alice, 2), 101),
			requireEnqueue(q, mm.NewSignedMessage(alice, 3), 102),
			requireEnqueue(q, mm.NewSignedMessage(bob, 1), 200),
		}

		assert.Equal(qm(msgs[0], 100), q.List(alice)[0])
		assert.Equal(qm(msgs[3], 200), q.List(bob)[0])

		root := blocks.NewBlock(0)
		root.Height = 100

		// Skip exactly 10 rounds since alice's first message enqueued
		b1 := blocks.NewBlock(1, root)
		b1.Height = 110
		err := policy.OnNewHeadTipset(ctx, requireTipset(t, root), requireTipset(t, b1))
		require.NoError(err)

		assert.Equal(qm(msgs[0], 100), q.List(alice)[0]) // No change
		assert.Equal(qm(msgs[3], 200), q.List(bob)[0])

		b2 := blocks.NewBlock(2, b1) // Height 111
		err = policy.OnNewHeadTipset(ctx, requireTipset(t, b1), requireTipset(t, b2))
		require.NoError(err)
		assert.Empty(q.List(alice))                    // Alice's messages all expired
		assert.Equal(qm(msgs[3], 200), q.List(bob)[0]) // Bob's remain
	})

	t.Run("fails when messages out of nonce order", func(t *testing.T) {
		blocks := chain.NewFakeBlockProvider()
		q := core.NewMessageQueue()
		policy := core.NewMessageQueuePolicy(q, blocks, 10)

		msgs := []*types.SignedMessage{
			requireEnqueue(q, mm.NewSignedMessage(alice, 1), 100),
			requireEnqueue(q, mm.NewSignedMessage(alice, 2), 101),
			requireEnqueue(q, mm.NewSignedMessage(alice, 3), 102),
		}

		root := blocks.NewBlock(0)
		root.Height = 100

		b1 := blocks.NewBlockWithMessages(1, []*types.SignedMessage{msgs[1]}, root)
		err := policy.OnNewHeadTipset(ctx, requireTipset(t, root), requireTipset(t, b1))
		require.Error(err)
		assert.Contains(err.Error(), "nonce 1, expected 2")
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
