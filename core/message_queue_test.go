package core_test

import (
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
	assert := assert.New(t)
	require := require.New(t)

	keys := types.MustGenerateKeyInfo(2, types.GenerateKeyInfoSeed())
	mm := types.NewMessageMaker(t, keys)

	alice := mm.Addresses()[0]
	bob := mm.Addresses()[1]
	require.NotEqual(alice, bob)

	requireEnqueue := func(q *core.MessageQueue, msg *types.SignedMessage, stamp uint64) {
		err := q.Enqueue(msg, stamp)
		require.NoError(err)
	}

	requireRemoveNext := func(q *core.MessageQueue, sender address.Address, expected uint64) *types.SignedMessage {
		msg, found, e := q.RemoveNext(sender, expected)
		require.True(found)
		require.NoError(e)
		return msg
	}

	assertLargestNonce := func(q *core.MessageQueue, sender address.Address, expected uint64) {
		largest, found := q.LargestNonce(sender)
		assert.True(found, "no messages")
		assert.Equal(expected, largest)
	}

	assertNoNonce := func(q *core.MessageQueue, sender address.Address) {
		_, found := q.LargestNonce(sender)
		assert.False(found, "unexpected messages")
	}

	t.Run("empty queue", func(t *testing.T) {
		q := core.NewMessageQueue()
		msg, found, err := q.RemoveNext(alice, 0)
		assert.Nil(msg)
		assert.False(found)
		assert.NoError(err)

		assert.Empty(q.ExpireBefore(math.MaxUint64))

		nonce, found := q.LargestNonce(alice)
		assert.False(found)
		assert.Zero(nonce)

		assert.Empty(q.List(alice))
		assert.Empty(q.Size())
	})

	t.Run("add and remove sequence", func(t *testing.T) {
		msgs := []*types.SignedMessage{
			mm.NewSignedMessage(alice, 0),
			mm.NewSignedMessage(alice, 1),
			mm.NewSignedMessage(alice, 2),
		}

		q := core.NewMessageQueue()
		assert.Equal(int64(0), q.Size())
		requireEnqueue(q, msgs[0], 0)
		requireEnqueue(q, msgs[1], 0)
		requireEnqueue(q, msgs[2], 0)
		assert.Equal(int64(3), q.Size())

		msg := requireRemoveNext(q, alice, 0)
		assert.Equal(msgs[0], msg)
		assert.Equal(int64(2), q.Size())

		_, found, err := q.RemoveNext(alice, 0) // Remove first message again
		assert.False(found)
		assert.NoError(err)
		assert.Equal(int64(2), q.Size())

		msg = requireRemoveNext(q, alice, 1)
		assert.Equal(msgs[1], msg)
		assert.Equal(int64(1), q.Size())

		_, found, err = q.RemoveNext(alice, 0) // Remove first message yet again
		assert.False(found)
		assert.NoError(err)
		assert.Equal(int64(1), q.Size())

		msg = requireRemoveNext(q, alice, 2)
		assert.Equal(msgs[2], msg)
		assert.Equal(int64(0), q.Size())
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

		err := q.Enqueue(msgs[0], 0) // Prior to existing
		assert.Error(err)

		err = q.Enqueue(msgs[1], 0) // Equal to existing
		assert.Error(err)

		err = q.Enqueue(msgs[3], 0) // Gap after existing
		assert.Error(err)
	})

	t.Run("invalid remove sequence", func(t *testing.T) {
		msgs := []*types.SignedMessage{
			mm.NewSignedMessage(alice, 10),
			mm.NewSignedMessage(alice, 11),
		}

		q := core.NewMessageQueue()
		requireEnqueue(q, msgs[0], 0)
		requireEnqueue(q, msgs[1], 0)

		msg, found, err := q.RemoveNext(alice, 9) // Prior to head
		assert.Nil(msg)
		assert.False(found)
		require.NoError(err)

		msg, found, err = q.RemoveNext(alice, 11) // After head
		assert.False(found)
		assert.Nil(msg)
		assert.Error(err)
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
		assert.Equal(int64(2), q.Size())
		assertLargestNonce(q, alice, 2)
		q.Clear(alice)
		assert.Equal(int64(0), q.Size())
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
		assert.Equal(int64(0), q.Size())

		requireEnqueue(q, fromAlice[0], 0)
		assertNoNonce(q, bob)
		assert.Equal(int64(1), q.Size())

		requireEnqueue(q, fromBob[0], 0)
		assertLargestNonce(q, alice, 0)
		assertLargestNonce(q, bob, 10)
		assert.Equal(int64(2), q.Size())

		requireEnqueue(q, fromBob[1], 0)
		requireEnqueue(q, fromBob[2], 0)
		assertLargestNonce(q, bob, 12)
		assert.Equal(int64(4), q.Size())

		requireEnqueue(q, fromAlice[1], 0)
		requireEnqueue(q, fromAlice[2], 0)
		assertLargestNonce(q, alice, 2)
		assert.Equal(int64(6), q.Size())

		msg := requireRemoveNext(q, alice, 0)
		assert.Equal(fromAlice[0], msg)
		assert.Equal(int64(5), q.Size())

		msg = requireRemoveNext(q, bob, 10)
		assert.Equal(fromBob[0], msg)
		assert.Equal(int64(4), q.Size())

		q.Clear(bob)
		assertLargestNonce(q, alice, 2)
		assertNoNonce(q, bob)
		assert.Equal(int64(2), q.Size())
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

		assert.Equal(&core.QueuedMessage{Msg: fromAlice[0], Stamp: 100}, q.List(alice)[0])
		assert.Equal(&core.QueuedMessage{Msg: fromBob[0], Stamp: 200}, q.List(bob)[0])

		expired := q.ExpireBefore(0)
		assert.Empty(expired)

		expired = q.ExpireBefore(100)
		assert.Empty(expired)

		// Alice's whole queue expires as soon as the first one does
		expired = q.ExpireBefore(101)
		assert.Equal(map[address.Address][]*types.SignedMessage{
			alice: {fromAlice[0], fromAlice[1]},
		}, expired)

		assert.Empty(q.List(alice))
		assertNoNonce(q, alice)
		assert.Equal(&core.QueuedMessage{Msg: fromBob[0], Stamp: 200}, q.List(bob)[0])
		assertLargestNonce(q, bob, 11)

		expired = q.ExpireBefore(300)
		assert.Equal(map[address.Address][]*types.SignedMessage{
			bob: {fromBob[0], fromBob[1]},
		}, expired)

		assert.Empty(q.List(bob))
		assertNoNonce(q, bob)
	})
}
