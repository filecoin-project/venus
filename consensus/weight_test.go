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
	"github.com/filecoin-project/go-filecoin/version"
)

func TestNewWeight(t *testing.T) {
	cst := hamt.NewCborStore()
	ctx := context.Background()
	fakeTree := state.TreeFromString(t, "test-NewWeight-StateCid", cst)
	fakeRoot, err := fakeTree.Flush(ctx)
	require.NoError(t, err)
	pvt, err := version.ConfigureProtocolVersions(version.TEST)
	require.NoError(t, err)
	// We only care about total power for the weight function
	// Total is 16, so bitlen is 5
	as := consensus.NewFakeActorStateStore(types.NewBytesAmount(1), types.NewBytesAmount(16), make(map[address.Address]address.Address))
	tickets := []types.Ticket{consensus.MakeFakeTicketForTest()}
	toWeigh := types.RequireNewTipSet(t, &types.Block{
		ParentWeight: 0,
		Tickets:      tickets,
	})
	sel := consensus.NewChainSelector(cst, as, types.CidFromString(t, "genesisCid"), pvt)

	t.Run("basic happy path", func(t *testing.T) {
		// 0 + 1[2*1 + 5] = 7
		fixWeight, err := sel.NewWeight(ctx, toWeigh, fakeRoot)
		assert.NoError(t, err)
		assertEqualInt(t, 7, fixWeight)
	})

	t.Run("total power adjusts as expected", func(t *testing.T) {
		asLowerX := consensus.NewFakeActorStateStore(types.NewBytesAmount(1), types.NewBytesAmount(15), make(map[address.Address]address.Address))
		asSameX := consensus.NewFakeActorStateStore(types.NewBytesAmount(1), types.NewBytesAmount(31), make(map[address.Address]address.Address))
		asHigherX := consensus.NewFakeActorStateStore(types.NewBytesAmount(1), types.NewBytesAmount(32), make(map[address.Address]address.Address))

		// Weight is 1 lower than total = 16 with total = 15
		selLower := consensus.NewChainSelector(cst, asLowerX, types.CidFromString(t, "genesisCid"), pvt)
		fixWeight, err := selLower.NewWeight(ctx, toWeigh, fakeRoot)
		assert.NoError(t, err)
		assertEqualInt(t, 6, fixWeight)

		// Weight is same as total = 16 with total = 31
		selSame := consensus.NewChainSelector(cst, asSameX, types.CidFromString(t, "genesisCid"), pvt)
		fixWeight, err = selSame.NewWeight(ctx, toWeigh, fakeRoot)
		assert.NoError(t, err)
		assertEqualInt(t, 7, fixWeight)

		// Weight is 1 higher than total = 16 with total = 32
		selHigher := consensus.NewChainSelector(cst, asHigherX, types.CidFromString(t, "genesisCid"), pvt)
		fixWeight, err = selHigher.NewWeight(ctx, toWeigh, fakeRoot)
		assert.NoError(t, err)
		assertEqualInt(t, 8, fixWeight)
	})

	t.Run("non-zero parent weight", func(t *testing.T) {
		parentWeight, err := types.BigToFixed(new(big.Float).SetInt64(int64(49)))
		require.NoError(t, err)
		toWeighWithParent := types.RequireNewTipSet(t, &types.Block{
			ParentWeight: types.Uint64(parentWeight),
			Tickets:      tickets,
		})

		// 49 + 1[2*1 + 5] = 56
		fixWeight, err := sel.NewWeight(ctx, toWeighWithParent, fakeRoot)
		assert.NoError(t, err)
		assertEqualInt(t, 56, fixWeight)
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
		fixWeight, err := sel.NewWeight(ctx, toWeighThreeBlock, fakeRoot)
		assert.NoError(t, err)
		assertEqualInt(t, 11, fixWeight)
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
		fixWeight, err := sel.NewWeight(ctx, toWeighTwoTickets, fakeRoot)
		assert.NoError(t, err)
		assertEqualInt(t, 7, fixWeight)
	})

	t.Run("many null", func(t *testing.T) {
		fifteenTickets := []types.Ticket{}
		expected := 1.0
		for i := 0; i < 15; i++ {
			fifteenTickets = append(fifteenTickets, consensus.MakeFakeTicketForTest())
			// consensus.pi = 0.87, expected = (pi^pn)(0.87)^15
			expected *= 0.87
		}
		toWeighFifteenNull := types.RequireNewTipSet(t, &types.Block{
			ParentWeight: 0,
			Tickets:      fifteenTickets,
		})

		// innerTerm = [2*1 + 5] = 7. expected = expected * innerterm
		expected *= 7
		bigExpected := big.NewFloat(expected)
		fixExpected, err := types.BigToFixed(bigExpected) // do fixed point rounding for cmp
		require.NoError(t, err)

		// 0 + ((0.87)^15)[2*1 + 5]
		fixWeight, err := sel.NewWeight(ctx, toWeighFifteenNull, fakeRoot)
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

// helper for asserting equality between int and fixed
func assertEqualInt(t *testing.T, i int, fixed uint64) {
	fixed2int := requireFixedToInt(t, fixed)
	assert.Equal(t, i, fixed2int)
}
