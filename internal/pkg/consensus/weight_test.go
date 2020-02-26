package consensus_test

import (
	"context"
	"math/big"
	"testing"

	"github.com/filecoin-project/specs-actors/actors/abi"
	fbig "github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	appstate "github.com/filecoin-project/go-filecoin/internal/pkg/state"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

func TestWeight(t *testing.T) {
	cst := cbor.NewMemCborStore()
	ctx := context.Background()
	fakeTree := state.NewFromString(t, "test-Weight-StateCid", cst)
	fakeRoot, err := fakeTree.Commit(ctx)
	require.NoError(t, err)
	// We only care about total power for the weight function
	// Total is 16, so bitlen is 5
	viewer := makeStateViewer(fakeRoot, abi.NewStoragePower(16))
	ticket := consensus.MakeFakeTicketForTest()
	toWeigh := th.RequireNewTipSet(t, &block.Block{
		ParentWeight: fbig.Zero(),
		Ticket:       ticket,
	})
	sel := consensus.NewChainSelector(cst, &viewer, types.CidFromString(t, "genesisCid"))

	t.Run("basic happy path", func(t *testing.T) {
		// 0 + 1[2*1 + 5] = 7
		fixWeight, err := sel.Weight(ctx, toWeigh, fakeRoot)
		assert.NoError(t, err)
		assertEqualInt(t, 7, fixWeight)
	})

	t.Run("total power adjusts as expected", func(t *testing.T) {
		asLowerX := makeStateViewer(fakeRoot, abi.NewStoragePower(15))
		asSameX := makeStateViewer(fakeRoot, abi.NewStoragePower(31))
		asHigherX := makeStateViewer(fakeRoot, abi.NewStoragePower(32))

		// Weight is 1 lower than total = 16 with total = 15
		selLower := consensus.NewChainSelector(cst, &asLowerX, types.CidFromString(t, "genesisCid"))
		fixWeight, err := selLower.Weight(ctx, toWeigh, fakeRoot)
		assert.NoError(t, err)
		assertEqualInt(t, 6, fixWeight)

		// Weight is same as total = 16 with total = 31
		selSame := consensus.NewChainSelector(cst, &asSameX, types.CidFromString(t, "genesisCid"))
		fixWeight, err = selSame.Weight(ctx, toWeigh, fakeRoot)
		assert.NoError(t, err)
		assertEqualInt(t, 7, fixWeight)

		// Weight is 1 higher than total = 16 with total = 32
		selHigher := consensus.NewChainSelector(cst, &asHigherX, types.CidFromString(t, "genesisCid"))
		fixWeight, err = selHigher.Weight(ctx, toWeigh, fakeRoot)
		assert.NoError(t, err)
		assertEqualInt(t, 8, fixWeight)
	})

	t.Run("non-zero parent weight", func(t *testing.T) {
		parentWeight, err := types.BigToFixed(new(big.Float).SetInt64(int64(49)))
		require.NoError(t, err)
		toWeighWithParent := th.RequireNewTipSet(t, &block.Block{
			ParentWeight: parentWeight,
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
				ParentWeight: fbig.Zero(),
				Ticket:       ticket,
				Timestamp:    0,
			},
			&block.Block{
				ParentWeight: fbig.Zero(),
				Ticket:       ticket,
				Timestamp:    1,
			},
			&block.Block{
				ParentWeight: fbig.Zero(),
				Ticket:       ticket,
				Timestamp:    2,
			},
		)
		// 0 + 1[2*3 + 5] = 11
		fixWeight, err := sel.Weight(ctx, toWeighThreeBlock, fakeRoot)
		assert.NoError(t, err)
		assertEqualInt(t, 11, fixWeight)
	})
}

func makeStateViewer(stateRoot cid.Cid, networkPower abi.StoragePower) consensus.FakePowerStateViewer {
	return consensus.FakePowerStateViewer{
		Views: map[cid.Cid]*appstate.FakeStateView{
			stateRoot: appstate.NewFakeStateView(networkPower),
		},
	}
}

// helper for turning fixed point reprs of int weights to ints
func requireFixedToInt(t *testing.T, fixedX fbig.Int) int {
	floatX, err := types.FixedToBig(fixedX)
	require.NoError(t, err)
	intX, _ := floatX.Int64()
	return int(intX)
}

// helper for asserting equality between int and fixed
func assertEqualInt(t *testing.T, i int, fixed fbig.Int) {
	fixed2int := requireFixedToInt(t, fixed)
	assert.Equal(t, i, fixed2int)
}
