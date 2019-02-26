package mining

import (
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
	"testing"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestMessageQueueOrder(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	var seed = types.GenerateKeyInfoSeed()
	var ki = types.MustGenerateKeyInfo(10, seed)
	var mockSigner = types.NewMockSigner(ki)

	a0 := mockSigner.Addresses[0]
	a1 := mockSigner.Addresses[2]
	a2 := mockSigner.Addresses[3]
	to := mockSigner.Addresses[9]

	sign := func(from address.Address, to address.Address, nonce uint64, units uint64, price int64) *types.SignedMessage {
		msg := types.Message{
			From:  from,
			To:    to,
			Nonce: types.Uint64(nonce),
		}
		s, err := types.NewSignedMessage(msg, &mockSigner, types.NewGasPrice(price), types.NewGasUnits(units))
		require.NoError(err)
		return s
	}

	t.Run("empty", func(t *testing.T) {
		q := NewMessageQueue([]*types.SignedMessage{})
		assert.True(q.Empty())
		msg, ok := q.Pop()
		assert.Nil(msg)
		assert.False(ok)
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
			last, seen := lastFromAddr[msg.From]
			if seen {
				assert.True(last <= uint64(msg.Nonce))
			}
			lastFromAddr[msg.From] = uint64(msg.Nonce)
		}
		assert.True(q.Empty())
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
		assert.Equal(expected, actual)
		assert.True(q.Empty())
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
		assert.Equal(expected, actual)
		assert.True(q.Empty())
	})
}
