package mining

import (
	"testing"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestSelectMessagesForBlock(t *testing.T) {
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
		ordered := SelectMessagesForBlock([]*types.SignedMessage{})
		assert.Equal(0, len(ordered))
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

		ordered := SelectMessagesForBlock(msgs)
		assert.Equal(len(msgs), len(ordered))

		lastFromAddr := make(map[address.Address]uint64)
		for _, m := range ordered {
			last, seen := lastFromAddr[m.From]
			if seen {
				assert.True(last <= uint64(m.Nonce))
			}
			lastFromAddr[m.From] = uint64(m.Nonce)
		}
	})
}
