package core_test

import (
	"fmt"
	"math"
	"testing"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestMessageQueue(t *testing.T) {
	t.Parallel() // Individual tests share msgSeq so not parallel (but quick)
	assert := assert.New(t)
	require := require.New(t)

	keys := types.MustGenerateKeyInfo(3, types.GenerateKeyInfoSeed())
	addresses := make([]address.Address, len(keys))
	signer := types.NewMockSigner(keys)
	msgSeq := 0
	gasPrice := *types.ZeroAttoFIL
	gasUnits := types.NewGasUnits(0)

	for i, key := range keys {
		addr, _ := key.Address()
		addresses[i] = addr
	}

	alice := addresses[0]
	bob := addresses[1]
	require.NotEqual(alice, bob)

	// Creates a new message with specified sender and nonce
	newMessage := func(from address.Address, nonce uint64) *types.SignedMessage {
		seq := msgSeq
		msgSeq++
		to, err := address.NewActorAddress([]byte("destination"))
		require.NoError(err)
		msg := types.NewMessage(
			from,
			to,
			nonce,
			types.NewAttoFILFromFIL(0),
			"method"+fmt.Sprintf("%d", seq),
			[]byte("params"))
		signed, err := types.NewSignedMessage(*msg, signer, gasPrice, gasUnits)
		require.NoError(err)
		return signed
	}

	mustEnqueue := func(q *core.MessageQueue, msg *types.SignedMessage, stamp uint64) {
		err := q.Enqueue(msg, stamp)
		require.NoError(err)
	}

	mustRemoveNext := func(q *core.MessageQueue, sender address.Address, expected uint64) *types.SignedMessage {
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

		assert.Equal(uint64(0), q.NextStamp(alice))
	})

	t.Run("add and remove sequence", func(t *testing.T) {
		msgs := []*types.SignedMessage{
			newMessage(alice, 0),
			newMessage(alice, 1),
			newMessage(alice, 2),
		}

		q := core.NewMessageQueue()
		mustEnqueue(q, msgs[0], 0)
		mustEnqueue(q, msgs[1], 0)
		mustEnqueue(q, msgs[2], 0)

		msg := mustRemoveNext(q, alice, 0)
		assert.Equal(msgs[0], msg)

		_, found, err := q.RemoveNext(alice, 0) // Remove first message again
		assert.False(found)
		assert.NoError(err)

		msg = mustRemoveNext(q, alice, 1)
		assert.Equal(msgs[1], msg)

		_, found, err = q.RemoveNext(alice, 0) // Remove first message yet again
		assert.False(found)
		assert.NoError(err)

		msg = mustRemoveNext(q, alice, 2)
		assert.Equal(msgs[2], msg)
	})

	t.Run("invalid nonce sequence", func(t *testing.T) {
		msgs := []*types.SignedMessage{
			newMessage(alice, 0),
			newMessage(alice, 1),
			newMessage(alice, 2),
			newMessage(alice, 3),
		}

		q := core.NewMessageQueue()
		mustEnqueue(q, msgs[1], 0)

		err := q.Enqueue(msgs[0], 0) // Prior to existing
		assert.Error(err)

		err = q.Enqueue(msgs[1], 0) // Equal to existing
		assert.Error(err)

		err = q.Enqueue(msgs[3], 0) // Gap after existing
		assert.Error(err)
	})

	t.Run("invalid remove sequence", func(t *testing.T) {
		msgs := []*types.SignedMessage{
			newMessage(alice, 10),
			newMessage(alice, 11),
		}

		q := core.NewMessageQueue()
		mustEnqueue(q, msgs[0], 0)
		mustEnqueue(q, msgs[1], 0)

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
			newMessage(alice, 0),
			newMessage(alice, 1),
			newMessage(alice, 2),
			newMessage(alice, 3),
		}
		q := core.NewMessageQueue()
		mustEnqueue(q, msgs[0], 0)
		assertLargestNonce(q, alice, 0)
		mustEnqueue(q, msgs[1], 0)
		assertLargestNonce(q, alice, 1)
		mustEnqueue(q, msgs[2], 0)
		assertLargestNonce(q, alice, 2)

		mustRemoveNext(q, alice, 0)
		assertLargestNonce(q, alice, 2)

		mustEnqueue(q, msgs[3], 0)
		assertLargestNonce(q, alice, 3)

		mustRemoveNext(q, alice, 1)
		mustRemoveNext(q, alice, 2)
		assertLargestNonce(q, alice, 3)

		mustRemoveNext(q, alice, 3) // clears queue
		assertNoNonce(q, alice)
	})

	t.Run("clear", func(t *testing.T) {
		msgs := []*types.SignedMessage{
			newMessage(alice, 0),
			newMessage(alice, 1),
			newMessage(alice, 2),
		}

		q := core.NewMessageQueue()
		mustEnqueue(q, msgs[1], 0)
		mustEnqueue(q, msgs[2], 0)
		assertLargestNonce(q, alice, 2)
		q.Clear(alice)
		assertNoNonce(q, alice)

		mustEnqueue(q, msgs[0], 0)
		mustEnqueue(q, msgs[1], 0)
		assertLargestNonce(q, alice, 1)
	})

	t.Run("independent addresses", func(t *testing.T) {
		fromAlice := []*types.SignedMessage{
			newMessage(alice, 0),
			newMessage(alice, 1),
			newMessage(alice, 2),
		}
		fromBob := []*types.SignedMessage{
			newMessage(bob, 10),
			newMessage(bob, 11),
			newMessage(bob, 12),
		}
		q := core.NewMessageQueue()

		mustEnqueue(q, fromAlice[0], 0)
		assertNoNonce(q, bob)

		mustEnqueue(q, fromBob[0], 0)
		assertLargestNonce(q, alice, 0)
		assertLargestNonce(q, bob, 10)

		mustEnqueue(q, fromBob[1], 0)
		mustEnqueue(q, fromBob[2], 0)
		assertLargestNonce(q, bob, 12)

		mustEnqueue(q, fromAlice[1], 0)
		mustEnqueue(q, fromAlice[2], 0)
		assertLargestNonce(q, alice, 2)

		msg := mustRemoveNext(q, alice, 0)
		assert.Equal(fromAlice[0], msg)

		msg = mustRemoveNext(q, bob, 10)
		assert.Equal(fromBob[0], msg)

		q.Clear(bob)
		assertLargestNonce(q, alice, 2)
		assertNoNonce(q, bob)
	})

	t.Run("expire before stamp", func(t *testing.T) {
		fromAlice := []*types.SignedMessage{
			newMessage(alice, 0),
			newMessage(alice, 1),
		}
		fromBob := []*types.SignedMessage{
			newMessage(bob, 10),
			newMessage(bob, 11),
		}
		q := core.NewMessageQueue()

		mustEnqueue(q, fromAlice[0], 100)
		mustEnqueue(q, fromAlice[1], 101)
		mustEnqueue(q, fromBob[0], 200)
		mustEnqueue(q, fromBob[1], 201)

		assert.Equal(uint64(100), q.NextStamp(alice))
		assert.Equal(uint64(200), q.NextStamp(bob))

		expired := q.ExpireBefore(0)
		assert.Empty(expired)

		expired = q.ExpireBefore(100)
		assert.Empty(expired)

		// Alice's whole queue expires as soon as the first one does
		expired = q.ExpireBefore(101)
		assert.Equal(map[address.Address][]*types.SignedMessage{
			alice: {fromAlice[0], fromAlice[1]},
		}, expired)

		assert.Equal(uint64(0), q.NextStamp(alice))
		assertNoNonce(q, alice)
		assert.Equal(uint64(200), q.NextStamp(bob))
		assertLargestNonce(q, bob, 11)

		expired = q.ExpireBefore(300)
		assert.Equal(map[address.Address][]*types.SignedMessage{
			bob: {fromBob[0], fromBob[1]},
		}, expired)

		assert.Equal(uint64(0), q.NextStamp(bob))
		assertNoNonce(q, bob)
	})
}
