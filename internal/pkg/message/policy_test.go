package message_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
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

	requireEnqueue := func(q *message.Queue, msg *types.SignedMessage, stamp uint64) *types.SignedMessage {
		err := q.Enqueue(ctx, msg, stamp)
		require.NoError(t, err)
		return msg
	}

	t.Run("old block does nothing", func(t *testing.T) {
		blocks := chain.NewBuilder(t, alice)
		q := message.NewQueue()
		policy := message.NewMessageQueuePolicy(blocks, 10)

		fromAlice := mm.NewSignedMessage(alice, 1)
		fromBob := mm.NewSignedMessage(bob, 1)
		requireEnqueue(q, fromAlice, 100)
		requireEnqueue(q, fromBob, 200)

		root := blocks.NewGenesis() // Height = 0
		b1 := blocks.AppendOn(root, 1)

		err := policy.HandleNewHead(ctx, q, nil, []block.TipSet{b1})
		assert.NoError(t, err)
		assert.Equal(t, qm(fromAlice, 100), q.List(alice)[0])
		assert.Equal(t, qm(fromBob, 200), q.List(bob)[0])
	})

	t.Run("chain truncation does nothing", func(t *testing.T) {
		blocks := chain.NewBuilder(t, alice)
		q := message.NewQueue()
		policy := message.NewMessageQueuePolicy(blocks, 10)

		fromAlice := mm.NewSignedMessage(alice, 1)
		fromBob := mm.NewSignedMessage(bob, 1)
		requireEnqueue(q, fromAlice, 100)
		requireEnqueue(q, fromBob, 200)

		root := blocks.NewGenesis() // Height = 0
		b1 := blocks.AppendOn(root, 1)

		err := policy.HandleNewHead(ctx, q, []block.TipSet{b1}, []block.TipSet{})
		assert.NoError(t, err)
		assert.Equal(t, qm(fromAlice, 100), q.List(alice)[0])
		assert.Equal(t, qm(fromBob, 200), q.List(bob)[0])
	})

	t.Run("removes mined messages", func(t *testing.T) {
		blocks := chain.NewBuilder(t, alice)
		q := message.NewQueue()
		policy := message.NewMessageQueuePolicy(blocks, 10)

		msgs := []*types.SignedMessage{
			requireEnqueue(q, mm.NewSignedMessage(alice, 1), 100),
			requireEnqueue(q, mm.NewSignedMessage(alice, 2), 101),
			requireEnqueue(q, mm.NewSignedMessage(alice, 3), 102),
			requireEnqueue(q, mm.NewSignedMessage(bob, 1), 100),
		}

		assert.Equal(t, qm(msgs[0], 100), q.List(alice)[0])
		assert.Equal(t, qm(msgs[3], 100), q.List(bob)[0])

		root := blocks.BuildOneOn(block.UndefTipSet, func(b *chain.BlockBuilder) {
			b.IncHeight(103)
		})
		b1 := blocks.BuildOneOn(root, func(b *chain.BlockBuilder) {
			b.AddMessages(
				[]*types.SignedMessage{msgs[0]},
				[]*types.UnsignedMessage{},
			)
		})

		err := policy.HandleNewHead(ctx, q, nil, []block.TipSet{b1})
		require.NoError(t, err)
		assert.Equal(t, qm(msgs[1], 101), q.List(alice)[0]) // First message removed successfully
		assert.Equal(t, qm(msgs[3], 100), q.List(bob)[0])   // No change

		// A block with no messages does nothing
		b2 := blocks.AppendOn(b1, 1)
		err = policy.HandleNewHead(ctx, q, []block.TipSet{}, []block.TipSet{b2})
		require.NoError(t, err)
		assert.Equal(t, qm(msgs[1], 101), q.List(alice)[0])
		assert.Equal(t, qm(msgs[3], 100), q.List(bob)[0])

		// Block with both alice and bob's next message
		b3 := blocks.BuildOneOn(b2, func(b *chain.BlockBuilder) {
			b.AddMessages(
				[]*types.SignedMessage{msgs[1], msgs[3]},
				[]*types.UnsignedMessage{},
			)
		})
		err = policy.HandleNewHead(ctx, q, nil, []block.TipSet{b3})
		require.NoError(t, err)
		assert.Equal(t, qm(msgs[2], 102), q.List(alice)[0])
		assert.Empty(t, q.List(bob)) // None left

		// Block with alice's last message
		b4 := blocks.BuildOneOn(b3, func(b *chain.BlockBuilder) {
			b.AddMessages(
				[]*types.SignedMessage{msgs[2]},
				[]*types.UnsignedMessage{},
			)
		})
		err = policy.HandleNewHead(ctx, q, nil, []block.TipSet{b4})
		require.NoError(t, err)
		assert.Empty(t, q.List(alice))
	})

	t.Run("expires old messages", func(t *testing.T) {
		blocks := chain.NewBuilder(t, alice)
		messages := blocks
		q := message.NewQueue()
		policy := message.NewMessageQueuePolicy(messages, 10)

		msgs := []*types.SignedMessage{
			requireEnqueue(q, mm.NewSignedMessage(alice, 1), 100),
			requireEnqueue(q, mm.NewSignedMessage(alice, 2), 101),
			requireEnqueue(q, mm.NewSignedMessage(alice, 3), 102),
			requireEnqueue(q, mm.NewSignedMessage(bob, 1), 200),
		}

		assert.Equal(t, qm(msgs[0], 100), q.List(alice)[0])
		assert.Equal(t, qm(msgs[3], 200), q.List(bob)[0])

		root := blocks.BuildOneOn(block.UndefTipSet, func(b *chain.BlockBuilder) {
			b.IncHeight(100)
		})

		// Skip 9 rounds since alice's first message enqueued, so b1 has height 110
		b1 := blocks.BuildOneOn(root, func(b *chain.BlockBuilder) {
			b.IncHeight(9)
		})

		err := policy.HandleNewHead(ctx, q, nil, []block.TipSet{b1})
		require.NoError(t, err)

		assert.Equal(t, qm(msgs[0], 100), q.List(alice)[0]) // No change
		assert.Equal(t, qm(msgs[3], 200), q.List(bob)[0])

		b2 := blocks.AppendOn(b1, 1) // Height b1.Height + 1 = 111
		err = policy.HandleNewHead(ctx, q, nil, []block.TipSet{b2})
		require.NoError(t, err)
		assert.Empty(t, q.List(alice))                    // Alice's messages all expired
		assert.Equal(t, qm(msgs[3], 200), q.List(bob)[0]) // Bob's remain
	})

	t.Run("fails when messages out of nonce order", func(t *testing.T) {
		blocks := chain.NewBuilder(t, alice)
		messages := blocks
		q := message.NewQueue()
		policy := message.NewMessageQueuePolicy(messages, 10)

		msgs := []*types.SignedMessage{
			requireEnqueue(q, mm.NewSignedMessage(alice, 1), 100),
			requireEnqueue(q, mm.NewSignedMessage(alice, 2), 101),
			requireEnqueue(q, mm.NewSignedMessage(alice, 3), 102),
		}

		root := blocks.BuildOneOn(block.UndefTipSet, func(b *chain.BlockBuilder) {
			b.IncHeight(100)
		})

		b1 := blocks.BuildOneOn(root, func(b *chain.BlockBuilder) {
			b.AddMessages(
				[]*types.SignedMessage{msgs[1]},
				[]*types.UnsignedMessage{},
			)
		})
		err := policy.HandleNewHead(ctx, q, nil, []block.TipSet{b1})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nonce 1, expected 2")
	})
}

func qm(msg *types.SignedMessage, stamp uint64) *message.Queued {
	return &message.Queued{Msg: msg, Stamp: stamp}
}
