package consensus_test

import (
	"context"
	"math/big"
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

func TestWeight(t *testing.T) {
	cst := hamt.NewCborStore()
	ctx := context.Background()
	fakeTree := state.TreeFromString(t, "test-Weight-StateCid", cst)
	fakeRoot, err := fakeTree.Flush(ctx)
	require.NoError(t, err)
	// We only care about total power for the weight function
	// Total is 16, so bitlen is 5
	as := consensus.NewFakeActorStateStore(types.NewBytesAmount(1), types.NewBytesAmount(16), make(map[address.Address]address.Address))
	ticket := consensus.MakeFakeTicketForTest()
	toWeigh := th.RequireNewTipSet(t, &block.Block{
		ParentWeight: 0,
		Ticket:       ticket,
	})
	sel := consensus.NewChainSelector(cst, as, types.CidFromString(t, "genesisCid"))

	t.Run("basic happy path", func(t *testing.T) {
		// 0 + 1[2*1 + 5] = 7
		fixWeight, err := sel.Weight(ctx, toWeigh, fakeRoot)
		assert.NoError(t, err)
		assertEqualInt(t, 7, fixWeight)
	})

	t.Run("total power adjusts as expected", func(t *testing.T) {
		asLowerX := consensus.NewFakeActorStateStore(types.NewBytesAmount(1), types.NewBytesAmount(15), make(map[address.Address]address.Address))
		asSameX := consensus.NewFakeActorStateStore(types.NewBytesAmount(1), types.NewBytesAmount(31), make(map[address.Address]address.Address))
		asHigherX := consensus.NewFakeActorStateStore(types.NewBytesAmount(1), types.NewBytesAmount(32), make(map[address.Address]address.Address))

		// Weight is 1 lower than total = 16 with total = 15
		selLower := consensus.NewChainSelector(cst, asLowerX, types.CidFromString(t, "genesisCid"))
		fixWeight, err := selLower.Weight(ctx, toWeigh, fakeRoot)
		assert.NoError(t, err)
		assertEqualInt(t, 6, fixWeight)

		// Weight is same as total = 16 with total = 31
		selSame := consensus.NewChainSelector(cst, asSameX, types.CidFromString(t, "genesisCid"))
		fixWeight, err = selSame.Weight(ctx, toWeigh, fakeRoot)
		assert.NoError(t, err)
		assertEqualInt(t, 7, fixWeight)

		// Weight is 1 higher than total = 16 with total = 32
		selHigher := consensus.NewChainSelector(cst, asHigherX, types.CidFromString(t, "genesisCid"))
		fixWeight, err = selHigher.Weight(ctx, toWeigh, fakeRoot)
		assert.NoError(t, err)
		assertEqualInt(t, 8, fixWeight)
	})

	t.Run("non-zero parent weight", func(t *testing.T) {
		parentWeight, err := types.BigToFixed(new(big.Float).SetInt64(int64(49)))
		require.NoError(t, err)
		toWeighWithParent := th.RequireNewTipSet(t, &block.Block{
			ParentWeight: types.Uint64(parentWeight),
			Ticket:       ticket,
		})

		// 49 + 1[2*1 + 5] = 56
		fixWeight, err := sel.Weight(ctx, toWeighWithParent, fakeRoot)
		assert.NoError(t, err)
		assertEqualInt(t, 56, fixWeight)
	})

	t.Run("many blocks", func(t *testing.T) {
		toWeighThreeBlock := th.RequireNewTipSet(t,
			&block.Block{
				ParentWeight: 0,
				Ticket:       ticket,
				Timestamp:    types.Uint64(0),
			},
			&block.Block{
				ParentWeight: 0,
				Ticket:       ticket,
				Timestamp:    types.Uint64(1),
			},
			&block.Block{
				ParentWeight: 0,
				Ticket:       ticket,
				Timestamp:    types.Uint64(2),
			},
		)
		// 0 + 1[2*3 + 5] = 11
		fixWeight, err := sel.Weight(ctx, toWeighThreeBlock, fakeRoot)
		assert.NoError(t, err)
		assertEqualInt(t, 11, fixWeight)
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
