package chain_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/venus/pkg/testhelpers"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/pkg/chain"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func TestIterAncestors(t *testing.T) {
	tf.UnitTest(t)
	miner, err := address.NewSecp256k1Address([]byte("address"))
	require.NoError(t, err)

	t.Run("iterates", func(t *testing.T) {
		ctx := context.Background()
		store := chain.NewBuilder(t, miner)

		root := store.Genesis().At(0)
		b11 := store.AppendBlockOnBlocks(ctx, root)
		b12 := store.AppendBlockOnBlocks(ctx, root)
		b21 := store.AppendBlockOnBlocks(ctx, b11, b12)

		t0 := testhelpers.RequireNewTipSet(t, root)
		t1 := testhelpers.RequireNewTipSet(t, b11, b12)
		t2 := testhelpers.RequireNewTipSet(t, b21)

		it := chain.IterAncestors(ctx, store, t2)
		assert.False(t, it.Complete())
		assert.True(t, t2.Equals(it.Value()))

		assert.NoError(t, it.Next(ctx))
		assert.False(t, it.Complete())
		assert.True(t, t1.Equals(it.Value()))

		assert.NoError(t, it.Next(ctx))
		assert.False(t, it.Complete())
		assert.True(t, t0.Equals(it.Value()))

		assert.NoError(t, it.Next(ctx))
		assert.True(t, it.Complete())
	})

	t.Run("respects context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		store := chain.NewBuilder(t, miner)

		root := store.Genesis().At(0)
		b11 := store.AppendBlockOnBlocks(ctx, root)
		b12 := store.AppendBlockOnBlocks(ctx, root)
		b21 := store.AppendBlockOnBlocks(ctx, b11, b12)

		testhelpers.RequireNewTipSet(t, root)
		t1 := testhelpers.RequireNewTipSet(t, b11, b12)
		t2 := testhelpers.RequireNewTipSet(t, b21)

		it := chain.IterAncestors(ctx, store, t2)
		assert.False(t, it.Complete())
		assert.True(t, t2.Equals(it.Value()))

		assert.NoError(t, it.Next(ctx))
		assert.False(t, it.Complete())
		assert.True(t, t1.Equals(it.Value()))

		cancel()

		assert.Error(t, it.Next(ctx))
	})
}

// Happy path
func TestCollectTipSetsOfHeightAtLeast(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)

	chainLen := 15
	head := builder.AppendManyOn(ctx, chainLen, builder.Genesis())

	stopHeight := abi.ChainEpoch(4)
	iterator := chain.IterAncestors(ctx, builder, head)
	tipsets, err := chain.CollectTipSetsOfHeightAtLeast(ctx, iterator, stopHeight)
	assert.NoError(t, err)
	latestHeight := tipsets[0].Height()
	assert.Equal(t, abi.ChainEpoch(15), latestHeight)
	earliestHeight := tipsets[len(tipsets)-1].Height()
	assert.Equal(t, abi.ChainEpoch(4), earliestHeight)
	assert.Equal(t, 12, len(tipsets))
}

// Height at least 0.
func TestCollectTipSetsOfHeightAtLeastZero(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)

	chainLen := 25
	head := builder.AppendManyOn(ctx, chainLen, builder.Genesis())

	stopHeight := abi.ChainEpoch(0)
	iterator := chain.IterAncestors(ctx, builder, head)
	tipsets, err := chain.CollectTipSetsOfHeightAtLeast(ctx, iterator, stopHeight)
	assert.NoError(t, err)
	latestHeight := tipsets[0].Height()
	assert.Equal(t, abi.ChainEpoch(25), latestHeight)
	earliestHeight := tipsets[len(tipsets)-1].Height()
	assert.Equal(t, abi.ChainEpoch(0), earliestHeight)
	assert.Equal(t, chainLen+1, len(tipsets))
}

// The starting epoch is a null block.
func TestCollectTipSetsOfHeightAtLeastStartingEpochIsNull(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	head := builder.Genesis()

	// Add 30 tipsets to the head of the chainStore.
	head = builder.AppendManyOn(ctx, 30, head)

	// Now add 10 null blocks and 1 tipset.
	head = builder.BuildOneOn(ctx, head, func(b *chain.BlockBuilder) {
		b.IncHeight(10)
	})

	// Now add 19 more tipsets.
	head = builder.AppendManyOn(ctx, 19, head)

	stopHeight := abi.ChainEpoch(35)
	iterator := chain.IterAncestors(ctx, builder, head)
	tipsets, err := chain.CollectTipSetsOfHeightAtLeast(ctx, iterator, stopHeight)
	assert.NoError(t, err)
	latestHeight := tipsets[0].Height()
	assert.Equal(t, abi.ChainEpoch(60), latestHeight)
	earliestHeight := tipsets[len(tipsets)-1].Height()
	assert.Equal(t, abi.ChainEpoch(41), earliestHeight)
	assert.Equal(t, 20, len(tipsets))
}

func TestFindCommonAncestorSameChain(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	head := builder.Genesis()
	// Add 30 tipsets to the head of the chainStore.
	head = builder.AppendManyOn(ctx, 30, head)
	headIterOne := chain.IterAncestors(ctx, builder, head)
	headIterTwo := chain.IterAncestors(ctx, builder, head)
	commonAncestor, err := chain.FindCommonAncestor(ctx, headIterOne, headIterTwo)
	assert.NoError(t, err)
	assert.Equal(t, head, commonAncestor)
}

func TestFindCommonAncestorFork(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	head := builder.Genesis()

	// Add 3 tipsets to the head of the chainStore.
	commonHeadTip := builder.AppendManyOn(ctx, 3, head)

	// Grow the fork chain
	lenFork := 10
	forkHead := builder.AppendManyOn(ctx, lenFork, commonHeadTip)

	// Grow the main chain
	lenMainChain := 14
	mainHead := builder.AppendManyOn(ctx, lenMainChain, commonHeadTip)

	forkItr := chain.IterAncestors(ctx, builder, forkHead)
	mainItr := chain.IterAncestors(ctx, builder, mainHead)
	commonAncestor, err := chain.FindCommonAncestor(ctx, mainItr, forkItr)
	assert.NoError(t, err)
	assert.ObjectsAreEqualValues(commonHeadTip, commonAncestor)
}

func TestFindCommonAncestorNoFork(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	head := builder.Genesis()

	// Add 30 tipsets to the head of the chainStore.
	head = builder.AppendManyOn(ctx, 30, head)
	headIterOne := chain.IterAncestors(ctx, builder, head)

	// Now add 19 more tipsets.
	expectedAncestor := head
	head = builder.AppendManyOn(ctx, 19, head)
	headIterTwo := chain.IterAncestors(ctx, builder, head)

	commonAncestor, err := chain.FindCommonAncestor(ctx, headIterOne, headIterTwo)
	assert.NoError(t, err)
	assert.True(t, expectedAncestor.Equals(commonAncestor))
}

// This test exercises an edge case fork that our previous common ancestor
// utility handled incorrectly.
func TestFindCommonAncestorNullBlockFork(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	head := builder.Genesis()

	// Add 10 tipsets to the head of the chainStore.
	commonHead := builder.AppendManyOn(ctx, 10, head)

	// From the common ancestor, add a block following a null block.
	headAfterNull := builder.BuildOneOn(ctx, commonHead, func(b *chain.BlockBuilder) {
		b.IncHeight(1)
	})
	afterNullItr := chain.IterAncestors(ctx, builder, headAfterNull)

	// Add a block (with no null) on another fork.
	headNoNull := builder.AppendOn(ctx, commonHead, 1)
	noNullItr := chain.IterAncestors(ctx, builder, headNoNull)

	commonAncestor, err := chain.FindCommonAncestor(ctx, afterNullItr, noNullItr)
	assert.NoError(t, err)
	assert.ObjectsAreEqualValues(commonHead, commonAncestor)
}
