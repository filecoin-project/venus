package core_test

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestMessageQueue(t *testing.T) {
	tf.UnitTest(t)

	// Individual tests share a MessageMaker so not parallel (but quick)
	keys := types.MustGenerateKeyInfo(2, 42)
	mm := types.NewMessageMaker(t, keys)

	alice := mm.Addresses()[0]
	bob := mm.Addresses()[1]
	require.NotEqual(t, alice, bob)

	ctx := context.Background()

	requireEnqueue := func(q *core.MessageQueue, msg *types.SignedMessage, stamp uint64) {
		err := q.Enqueue(ctx, msg, stamp)
		require.NoError(t, err)
	}

	requireRemoveNext := func(q *core.MessageQueue, sender address.Address, expected uint64) *types.SignedMessage {
		msg, found, e := q.RemoveNext(ctx, sender, expected)
		require.True(t, found)
		require.NoError(t, e)
		return msg
	}

	assertLargestNonce := func(q *core.MessageQueue, sender address.Address, expected uint64) {
		largest, found := q.LargestNonce(sender)
		assert.True(t, found, "no messages")
		assert.Equal(t, expected, largest)
	}

	assertNoNonce := func(q *core.MessageQueue, sender address.Address) {
		_, found := q.LargestNonce(sender)
		assert.False(t, found, "unexpected messages")
	}

	t.Run("empty queue", func(t *testing.T) {
		q := core.NewMessageQueue()
		msg, found, err := q.RemoveNext(ctx, alice, 0)
		assert.Nil(t, msg)
		assert.False(t, found)
		assert.NoError(t, err)

		assert.Empty(t, q.ExpireBefore(ctx, math.MaxUint64))

		nonce, found := q.LargestNonce(alice)
		assert.False(t, found)
		assert.Zero(t, nonce)

		assert.Empty(t, q.List(alice))
		assert.Empty(t, q.Size())
	})

	t.Run("add and remove sequence", func(t *testing.T) {
		msgs := []*types.SignedMessage{
			mm.NewSignedMessage(alice, 0),
			mm.NewSignedMessage(alice, 1),
			mm.NewSignedMessage(alice, 2),
		}

		q := core.NewMessageQueue()
		assert.Equal(t, int64(0), q.Size())
		requireEnqueue(q, msgs[0], 0)
		requireEnqueue(q, msgs[1], 0)
		requireEnqueue(q, msgs[2], 0)
		assert.Equal(t, int64(3), q.Size())

		msg := requireRemoveNext(q, alice, 0)
		assert.Equal(t, msgs[0], msg)
		assert.Equal(t, int64(2), q.Size())

		_, found, err := q.RemoveNext(ctx, alice, 0) // Remove first message again
		assert.False(t, found)
		assert.NoError(t, err)
		assert.Equal(t, int64(2), q.Size())

		msg = requireRemoveNext(q, alice, 1)
		assert.Equal(t, msgs[1], msg)
		assert.Equal(t, int64(1), q.Size())

		_, found, err = q.RemoveNext(ctx, alice, 0) // Remove first message yet again
		assert.False(t, found)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), q.Size())

		msg = requireRemoveNext(q, alice, 2)
		assert.Equal(t, msgs[2], msg)
		assert.Equal(t, int64(0), q.Size())
	})

	t.Run("invalid nonce sequence", func(t *testing.T) {
		msgs := []*types.SignedMessage{
			mm.NewSignedMessage(alice, 0),
			mm.NewSignedMessage(alice, 1),
			mm.NewSignedMessage(alice, 2),
			mm.NewSignedMessage(alice, 3),
		}

		q := core.NewMessageQueue()
		requireEnqueue(q, msgs[1], 0)

		err := q.Enqueue(ctx, msgs[0], 0) // Prior to existing
		assert.Error(t, err)

		err = q.Enqueue(ctx, msgs[1], 0) // Equal to existing
		assert.Error(t, err)

		err = q.Enqueue(ctx, msgs[3], 0) // Gap after existing
		assert.Error(t, err)
	})

	t.Run("invalid remove sequence", func(t *testing.T) {
		msgs := []*types.SignedMessage{
			mm.NewSignedMessage(alice, 10),
			mm.NewSignedMessage(alice, 11),
		}

		q := core.NewMessageQueue()
		requireEnqueue(q, msgs[0], 0)
		requireEnqueue(q, msgs[1], 0)

		msg, found, err := q.RemoveNext(ctx, alice, 9) // Prior to head
		assert.Nil(t, msg)
		assert.False(t, found)
		require.NoError(t, err)

		msg, found, err = q.RemoveNext(ctx, alice, 11) // After head
		assert.False(t, found)
		assert.Nil(t, msg)
		assert.Error(t, err)
	})

	t.Run("largest nonce", func(t *testing.T) {
		msgs := []*types.SignedMessage{
			mm.NewSignedMessage(alice, 0),
			mm.NewSignedMessage(alice, 1),
			mm.NewSignedMessage(alice, 2),
			mm.NewSignedMessage(alice, 3),
		}
		q := core.NewMessageQueue()
		requireEnqueue(q, msgs[0], 0)
		assertLargestNonce(q, alice, 0)
		requireEnqueue(q, msgs[1], 0)
		assertLargestNonce(q, alice, 1)
		requireEnqueue(q, msgs[2], 0)
		assertLargestNonce(q, alice, 2)

		requireRemoveNext(q, alice, 0)
		assertLargestNonce(q, alice, 2)

		requireEnqueue(q, msgs[3], 0)
		assertLargestNonce(q, alice, 3)

		requireRemoveNext(q, alice, 1)
		requireRemoveNext(q, alice, 2)
		assertLargestNonce(q, alice, 3)

		requireRemoveNext(q, alice, 3) // clears queue
		assertNoNonce(q, alice)
	})

	t.Run("clear", func(t *testing.T) {
		msgs := []*types.SignedMessage{
			mm.NewSignedMessage(alice, 0),
			mm.NewSignedMessage(alice, 1),
			mm.NewSignedMessage(alice, 2),
		}

		q := core.NewMessageQueue()
		requireEnqueue(q, msgs[1], 0)
		requireEnqueue(q, msgs[2], 0)
		assert.Equal(t, int64(2), q.Size())
		assertLargestNonce(q, alice, 2)
		q.Clear(ctx, alice)
		assert.Equal(t, int64(0), q.Size())
		assertNoNonce(q, alice)

		requireEnqueue(q, msgs[0], 0)
		requireEnqueue(q, msgs[1], 0)
		assertLargestNonce(q, alice, 1)
	})

	t.Run("independent addresses", func(t *testing.T) {
		fromAlice := []*types.SignedMessage{
			mm.NewSignedMessage(alice, 0),
			mm.NewSignedMessage(alice, 1),
			mm.NewSignedMessage(alice, 2),
		}
		fromBob := []*types.SignedMessage{
			mm.NewSignedMessage(bob, 10),
			mm.NewSignedMessage(bob, 11),
			mm.NewSignedMessage(bob, 12),
		}
		q := core.NewMessageQueue()
		assert.Equal(t, int64(0), q.Size())

		requireEnqueue(q, fromAlice[0], 0)
		assertNoNonce(q, bob)
		assert.Equal(t, int64(1), q.Size())

		requireEnqueue(q, fromBob[0], 0)
		assertLargestNonce(q, alice, 0)
		assertLargestNonce(q, bob, 10)
		assert.Equal(t, int64(2), q.Size())

		requireEnqueue(q, fromBob[1], 0)
		requireEnqueue(q, fromBob[2], 0)
		assertLargestNonce(q, bob, 12)
		assert.Equal(t, int64(4), q.Size())

		requireEnqueue(q, fromAlice[1], 0)
		requireEnqueue(q, fromAlice[2], 0)
		assertLargestNonce(q, alice, 2)
		assert.Equal(t, int64(6), q.Size())

		msg := requireRemoveNext(q, alice, 0)
		assert.Equal(t, fromAlice[0], msg)
		assert.Equal(t, int64(5), q.Size())

		msg = requireRemoveNext(q, bob, 10)
		assert.Equal(t, fromBob[0], msg)
		assert.Equal(t, int64(4), q.Size())

		q.Clear(ctx, bob)
		assertLargestNonce(q, alice, 2)
		assertNoNonce(q, bob)
		assert.Equal(t, int64(2), q.Size())
	})

	t.Run("expire before stamp", func(t *testing.T) {
		fromAlice := []*types.SignedMessage{
			mm.NewSignedMessage(alice, 0),
			mm.NewSignedMessage(alice, 1),
		}
		fromBob := []*types.SignedMessage{
			mm.NewSignedMessage(bob, 10),
			mm.NewSignedMessage(bob, 11),
		}
		q := core.NewMessageQueue()

		requireEnqueue(q, fromAlice[0], 100)
		requireEnqueue(q, fromAlice[1], 101)
		requireEnqueue(q, fromBob[0], 200)
		requireEnqueue(q, fromBob[1], 201)

		assert.Equal(t, &core.QueuedMessage{Msg: fromAlice[0], Stamp: 100}, q.List(alice)[0])
		assert.Equal(t, &core.QueuedMessage{Msg: fromBob[0], Stamp: 200}, q.List(bob)[0])

		expired := q.ExpireBefore(ctx, 0)
		assert.Empty(t, expired)

		expired = q.ExpireBefore(ctx, 100)
		assert.Empty(t, expired)

		// Alice's whole queue expires as soon as the first one does
		expired = q.ExpireBefore(ctx, 101)
		assert.Equal(t, map[address.Address][]*types.SignedMessage{
			alice: {fromAlice[0], fromAlice[1]},
		}, expired)

		assert.Empty(t, q.List(alice))
		assertNoNonce(q, alice)
		assert.Equal(t, &core.QueuedMessage{Msg: fromBob[0], Stamp: 200}, q.List(bob)[0])
		assertLargestNonce(q, bob, 11)

		expired = q.ExpireBefore(ctx, 300)
		assert.Equal(t, map[address.Address][]*types.SignedMessage{
			bob: {fromBob[0], fromBob[1]},
		}, expired)

		assert.Empty(t, q.List(bob))
		assertNoNonce(q, bob)
	})

	t.Run("oldest is correct", func(t *testing.T) {
		fromAlice := []*types.SignedMessage{
			mm.NewSignedMessage(alice, 0),
			mm.NewSignedMessage(alice, 1),
		}
		fromBob := []*types.SignedMessage{
			mm.NewSignedMessage(bob, 10),
			mm.NewSignedMessage(bob, 11),
		}
		q := core.NewMessageQueue()

		assert.Equal(t, uint64(0), q.Oldest())

		requireEnqueue(q, fromAlice[0], 100)
		assert.Equal(t, uint64(100), q.Oldest())

		requireEnqueue(q, fromAlice[1], 101)
		assert.Equal(t, uint64(100), q.Oldest())

		requireEnqueue(q, fromBob[0], 99)
		assert.Equal(t, uint64(99), q.Oldest())

		requireEnqueue(q, fromBob[1], 1)
		assert.Equal(t, uint64(1), q.Oldest())

	})
}
