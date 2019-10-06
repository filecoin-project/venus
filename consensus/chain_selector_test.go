package consensus_test

import (
	"context"
	"math/big"
	"testing"

	"github.com/ipfs/go-hamt-ipld"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestNewWeight(t *testing.T) {
	cst := hamt.NewCborStore()
	ctx := context.Background()
	fakeTree := &state.MockStateTree{}
	// We only care about total power for the weight function
	// Total is 16, so bitlen is 5
	as := consensus.NewFakeActorStateStore(types.NewBytesAmount(1), types.NewBytesAmount(16), make(map[address.Address]address.Address))
	tickets := []types.Ticket{consensus.MakeFakeTicketForTest()}
	toWeigh := types.RequireNewTipSet(t, &types.Block{
		ParentWeight: 0,
		Tickets:      tickets,
	})
	sel := consensus.NewChainSelector(cst, as, types.CidFromString(t, "genesisCid"))

	t.Run("basic happy path", func(t *testing.T) {
		// 0 + 1[2*1 + 5] = 7
		fixWeight, err := sel.NewWeight(ctx, toWeigh, fakeTree)
		assert.NoError(t, err)
		intWeight := requireFixedToInt(t, fixWeight)
		assert.Equal(t, 7, intWeight)
	})

	t.Run("total power adjusts as expected", func(t *testing.T) {
		asLowerX := consensus.NewFakeActorStateStore(types.NewBytesAmount(1), types.NewBytesAmount(15), make(map[address.Address]address.Address))
		asSameX := consensus.NewFakeActorStateStore(types.NewBytesAmount(1), types.NewBytesAmount(31), make(map[address.Address]address.Address))
		asHigherX := consensus.NewFakeActorStateStore(types.NewBytesAmount(1), types.NewBytesAmount(32), make(map[address.Address]address.Address))

		// Weight is 1 lower than with total = 16 with total = 15
		selLower := consensus.NewChainSelector(cst, asLowerX, types.CidFromString(t, "genesisCid"))
		fixWeight, err := selLower.NewWeight(ctx, toWeigh, fakeTree)
		assert.NoError(t, err)
		intWeight := requireFixedToInt(t, fixWeight)
		assert.Equal(t, 6, intWeight)

		// Weight is same as total = 16 with total = 31
		selSame := consensus.NewChainSelector(cst, asSameX, types.CidFromString(t, "genesisCid"))
		fixWeight, err = selSame.NewWeight(ctx, toWeigh, fakeTree)
		assert.NoError(t, err)
		intWeight = requireFixedToInt(t, fixWeight)
		assert.Equal(t, 7, intWeight)

		// Weight is 1 higher than total = 16 with total = 32
		selHigher := consensus.NewChainSelector(cst, asHigherX, types.CidFromString(t, "genesisCid"))
		fixWeight, err = selHigher.NewWeight(ctx, toWeigh, fakeTree)
		assert.NoError(t, err)
		intWeight = requireFixedToInt(t, fixWeight)
		assert.Equal(t, 8, intWeight)
	})

	t.Run("non-zero parent weight", func(t *testing.T) {
		parentWeight, err := types.BigToFixed(new(big.Float).SetInt64(int64(49)))
		require.NoError(t, err)
		toWeighWithParent := types.RequireNewTipSet(t, &types.Block{
			ParentWeight: types.Uint64(parentWeight),
			Tickets:      tickets,
		})

		// 49 + 1[2*1 + 5] = 56
		fixWeight, err := sel.NewWeight(ctx, toWeighWithParent, fakeTree)
		assert.NoError(t, err)
		intWeight := requireFixedToInt(t, fixWeight)
		assert.Equal(t, 56, intWeight)
	})

	t.Run("many blocks", func(t *testing.T) {
		toWeighThreeBlock := types.RequireNewTipSet(t,
			&types.Block{
				ParentWeight: 0,
				Tickets:      tickets,
				Timestamp:    types.Uint64(0),
			},
			&types.Block{
				ParentWeight: 0,
				Tickets:      tickets,
				Timestamp:    types.Uint64(1),
			},
			&types.Block{
				ParentWeight: 0,
				Tickets:      tickets,
				Timestamp:    types.Uint64(2),
			},
		)
		// 0 + 1[2*3 + 5] = 11
		fixWeight, err := sel.NewWeight(ctx, toWeighThreeBlock, fakeTree)
		assert.NoError(t, err)
		intWeight := requireFixedToInt(t, fixWeight)
		assert.Equal(t, 11, intWeight)
	})

	t.Run("few null", func(t *testing.T) {
		twoTickets := []types.Ticket{
			consensus.MakeFakeTicketForTest(),
			consensus.MakeFakeTicketForTest(),
		}

		toWeighTwoTickets := types.RequireNewTipSet(t, &types.Block{
			ParentWeight: 0,
			Tickets:      twoTickets,
		})

		// 0 + 1[2*1 + 5] = 7
		fixWeight, err := sel.NewWeight(ctx, toWeighTwoTickets, fakeTree)
		assert.NoError(t, err)
		intWeight := requireFixedToInt(t, fixWeight)
		assert.Equal(t, 7, intWeight)
	})

	t.Run("many null", func(t *testing.T) {
		fifteenTickets := []types.Ticket{}
		expected := 1.0
		for i := 0; i < 15; i++ {
			fifteenTickets = append(fifteenTickets, consensus.MakeFakeTicketForTest())
			expected *= consensus.PI
		}
		toWeighFifteenNull := types.RequireNewTipSet(t, &types.Block{
			ParentWeight: 0,
			Tickets:      fifteenTickets,
		})

		expected *= 7
		bigExpected := big.NewFloat(expected)
		fixExpected, err := types.BigToFixed(bigExpected) // do fixed point rounding for cmp
		require.NoError(t, err)

		// 0 + ((0.87)^15)[2*1 + 5]
		fixWeight, err := sel.NewWeight(ctx, toWeighFifteenNull, fakeTree)
		assert.NoError(t, err)
		assert.Equal(t, fixExpected, fixWeight)
	})
}

// helper for turning fixed point reprs of int weights to ints
func requireFixedToInt(t *testing.T, fixedX uint64) int {
	floatX, err := types.FixedToBig(fixedX)
	require.NoError(t, err)
	intX, _ := floatX.Int64()
	return int(intX)
}
