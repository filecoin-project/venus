package mining

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

func TestMessageQueueOrder(t *testing.T) {
	tf.UnitTest(t)

	var ki = types.MustGenerateKeyInfo(10, 42)
	var mockSigner = types.NewMockSigner(ki)

	a0 := mockSigner.Addresses[0]
	a1 := mockSigner.Addresses[2]
	a2 := mockSigner.Addresses[3]
	to := mockSigner.Addresses[9]

	sign := func(from address.Address, to address.Address, nonce uint64, units uint64, price int64) *types.SignedMessage {
		msg := types.UnsignedMessage{
			From:       from,
			To:         to,
			CallSeqNum: types.Uint64(nonce),
			GasPrice:   types.NewGasPrice(price),
			GasLimit:   types.NewGasUnits(units),
		}
		s, err := types.NewSignedMessage(msg, &mockSigner)
		require.NoError(t, err)
		return s
	}

	t.Run("empty", func(t *testing.T) {
		q := NewMessageQueue([]*types.SignedMessage{})
		assert.True(t, q.Empty())
		msg, ok := q.Pop()
		assert.Nil(t, msg)
		assert.False(t, ok)
	})

	t.Run("orders by nonce", func(t *testing.T) {
		msgs := []*types.SignedMessage{
			// Msgs from a0 are in increasing order.
			// Msgs from a1 are in decreasing order.
			// Msgs from a2 are out of order.
			// Messages from different signers are interleaved.
			sign(a0, to, 0, 0, 0),
			sign(a1, to, 15, 0, 0),
			sign(a2, to, 5, 0, 0),

			sign(a0, to, 1, 0, 0),
			sign(a1, to, 2, 0, 0),
			sign(a2, to, 7, 0, 0),

			sign(a0, to, 20, 0, 0),
			sign(a1, to, 1, 0, 0),
			sign(a2, to, 1, 0, 0),
		}

		q := NewMessageQueue(msgs)

		lastFromAddr := make(map[address.Address]uint64)
		for msg, more := q.Pop(); more == true; msg, more = q.Pop() {
			last, seen := lastFromAddr[msg.Message.From]
			if seen {
				assert.True(t, last <= uint64(msg.Message.CallSeqNum))
			}
			lastFromAddr[msg.Message.From] = uint64(msg.Message.CallSeqNum)
		}
		assert.True(t, q.Empty())
	})

	t.Run("orders by gas price", func(t *testing.T) {
		msgs := []*types.SignedMessage{
			sign(a0, to, 0, 0, 2),
			sign(a1, to, 0, 0, 3),
			sign(a2, to, 0, 0, 1),
		}
		q := NewMessageQueue(msgs)
		expected := []*types.SignedMessage{msgs[1], msgs[0], msgs[2]}
		actual := q.Drain()
		assert.Equal(t, expected, actual)
		assert.True(t, q.Empty())
	})

	t.Run("nonce overrides gas price", func(t *testing.T) {
		msgs := []*types.SignedMessage{
			sign(a0, to, 0, 0, 1),
			sign(a0, to, 1, 0, 3), // More valuable but must come after previous message from a0
			sign(a2, to, 0, 0, 2),
		}
		expected := []*types.SignedMessage{msgs[2], msgs[0], msgs[1]}

		q := NewMessageQueue(msgs)
		actual := q.Drain()
		assert.Equal(t, expected, actual)
		assert.True(t, q.Empty())
	})
}
