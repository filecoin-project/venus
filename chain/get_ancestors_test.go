package chain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

// Happy path
func TestCollectTipSetsOfHeightAtLeast(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)

	chainLen := 15
	head := builder.AppendManyTo(chainLen)

	stopHeight := types.NewBlockHeight(uint64(4))
	iterator := chain.IterAncestors(ctx, builder, types.RequireNewTipSet(t, head))
	tipsets, err := chain.CollectTipSetsOfHeightAtLeast(ctx, iterator, stopHeight)
	assert.NoError(t, err)
	latestHeight, err := tipsets[0].Height()
	require.NoError(t, err)
	assert.Equal(t, uint64(14), latestHeight)
	earliestHeight, err := tipsets[len(tipsets)-1].Height()
	require.NoError(t, err)
	assert.Equal(t, uint64(4), earliestHeight)
	assert.Equal(t, 11, len(tipsets))
}

// Height at least 0.
func TestCollectTipSetsOfHeightAtLeastZero(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)

	chainLen := 25
	head := builder.AppendManyTo(chainLen)

	stopHeight := types.NewBlockHeight(uint64(0))
	iterator := chain.IterAncestors(ctx, builder, types.RequireNewTipSet(t, head))
	tipsets, err := chain.CollectTipSetsOfHeightAtLeast(ctx, iterator, stopHeight)
	assert.NoError(t, err)
	latestHeight, err := tipsets[0].Height()
	require.NoError(t, err)
	assert.Equal(t, uint64(24), latestHeight)
	earliestHeight, err := tipsets[len(tipsets)-1].Height()
	require.NoError(t, err)
	assert.Equal(t, uint64(0), earliestHeight)
	assert.Equal(t, chainLen, len(tipsets))
}

// The starting epoch is a null block.
func TestCollectTipSetsOfHeightAtLeastStartingEpochIsNull(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	head := builder.AppendTo()

	// Add 30 tipsets to the head of the chainStore.
	head = builder.AppendManyTo(30, head)

	// Now add 10 null blocks and 1 tipset.
	head = builder.BuildTo(head, func(b *chain.BlockBuilder) {
		b.IncHeight(10)
	})

	// Now add 19 more tipsets.
	head = builder.AppendManyTo(19, head)

	stopHeight := types.NewBlockHeight(uint64(35))
	iterator := chain.IterAncestors(ctx, builder, types.RequireNewTipSet(t, head))
	tipsets, err := chain.CollectTipSetsOfHeightAtLeast(ctx, iterator, stopHeight)
	assert.NoError(t, err)
	latestHeight, err := tipsets[0].Height()
	require.NoError(t, err)
	assert.Equal(t, uint64(60), latestHeight)
	earliestHeight, err := tipsets[len(tipsets)-1].Height()
	require.NoError(t, err)
	assert.Equal(t, uint64(41), earliestHeight)
	assert.Equal(t, 20, len(tipsets))
}

func TestCollectAtMostNTipSets(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)

	chainLen := 25
	head := builder.AppendManyTo(chainLen)

	t.Run("happy path", func(t *testing.T) {
		number := uint(10)
		iterator := chain.IterAncestors(ctx, builder, types.RequireNewTipSet(t, head))
		tipsets, err := chain.CollectAtMostNTipSets(ctx, iterator, number)
		assert.NoError(t, err)
		assert.Equal(t, 10, len(tipsets))
	})
	t.Run("hit genesis", func(t *testing.T) {
		number := uint(400)
		iterator := chain.IterAncestors(ctx, builder, types.RequireNewTipSet(t, head))
		tipsets, err := chain.CollectAtMostNTipSets(ctx, iterator, number)
		assert.NoError(t, err)
		assert.Equal(t, 25, len(tipsets))
	})
}

// Test the happy path.
// Make a chain of 200 tipsets
// DependentAncestor epochs = 100
// Lookback = 20
func TestGetRecentAncestors(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)

	chainLen := 200
	headBlock := builder.AppendManyTo(chainLen)
	head := types.RequireNewTipSet(t, headBlock)

	epochs := uint64(100)
	lookback := uint(20)
	ancestors, err := chain.GetRecentAncestors(ctx, head, builder,
		types.NewBlockHeight(uint64(headBlock.Height+1)), types.NewBlockHeight(epochs), lookback)
	require.NoError(t, err)
	assert.Equal(t, ancestors[0], head)
	assert.Equal(t, int(epochs)+int(lookback), len(ancestors))
	for i := 0; i < len(ancestors); i++ {
		h, err := ancestors[i].Height()
		assert.NoError(t, err)
		assert.Equal(t, h, uint64(chainLen-1-i))
	}
}

// Test case where parameters specify a chain past genesis.
func TestGetRecentAncestorsTruncates(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)

	chainLen := 100
	headBlock := builder.AppendManyTo(chainLen)
	head := types.RequireNewTipSet(t, headBlock)
	h, err := head.Height()
	require.NoError(t, err)
	epochs := uint64(200)
	lookback := uint(20)

	t.Run("more epochs than chainStore", func(t *testing.T) {
		ancestors, err := chain.GetRecentAncestors(ctx, head, builder, types.NewBlockHeight(h+uint64(1)), types.NewBlockHeight(epochs), lookback)
		require.NoError(t, err)
		assert.Equal(t, chainLen, len(ancestors))
	})

	t.Run("more epochs + lookback than chainStore", func(t *testing.T) {
		epochs = uint64(60)
		lookback = uint(50)
		ancestors, err := chain.GetRecentAncestors(ctx, head, builder, types.NewBlockHeight(h+uint64(1)), types.NewBlockHeight(epochs), lookback)
		require.NoError(t, err)
		assert.Equal(t, chainLen, len(ancestors))
	})
}

// Test case where no block has the start height in the chain due to null blocks.
func TestGetRecentAncestorsStartingEpochIsNull(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	head := builder.AppendTo()

	// Add 30 tipsets to the head of the chainStore.
	head = builder.AppendManyTo(30, head)
	// Add 10 null blocks and 1 tipset.
	head = builder.BuildTo(head, func(b *chain.BlockBuilder) {
		b.IncHeight(10)
	})
	// Add 19 more tipsets.
	len2 := 19
	head = builder.AppendManyTo(len2, head)

	epochs := uint64(28)
	lookback := 6
	headTipSet := types.RequireNewTipSet(t, head)
	h, err := headTipSet.Height()
	require.NoError(t, err)
	ancestors, err := chain.GetRecentAncestors(ctx, headTipSet, builder, types.NewBlockHeight(h+uint64(1)), types.NewBlockHeight(epochs), uint(lookback))
	require.NoError(t, err)

	// We expect to see 20 blocks in the first 28 epochs and an additional 6 for the lookback parameter
	assert.Equal(t, len2+lookback+1, len(ancestors))
	lastBlockHeight, err := ancestors[len(ancestors)-1].Height()
	require.NoError(t, err)
	assert.Equal(t, uint64(25), lastBlockHeight)
}

func TestFindCommonAncestorSameChain(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	head := builder.AppendTo()
	// Add 30 tipsets to the head of the chainStore.
	head = builder.AppendManyTo(30, head)
	headTipSet := types.RequireNewTipSet(t, head)
	headIterOne := chain.IterAncestors(ctx, builder, headTipSet)
	headIterTwo := chain.IterAncestors(ctx, builder, headTipSet)
	commonAncestor, err := chain.FindCommonAncestor(headIterOne, headIterTwo)
	assert.NoError(t, err)
	assert.Equal(t, headTipSet, commonAncestor)
}

func TestFindCommonAncestorFork(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	head := builder.AppendTo()

	// Add 3 tipsets to the head of the chainStore.
	commonHeadBlock := builder.AppendManyTo(3, head)
	commonHeadTip := types.RequireNewTipSet(t, commonHeadBlock)

	// Grow the fork chain
	lenFork := 10
	forkHead := builder.AppendManyTo(lenFork, commonHeadBlock)

	// Grow the main chain
	lenMainChain := 14
	mainHead := builder.AppendManyTo(lenMainChain, commonHeadBlock)

	forkItr := chain.IterAncestors(ctx, builder, types.RequireNewTipSet(t, forkHead))
	mainItr := chain.IterAncestors(ctx, builder, types.RequireNewTipSet(t, mainHead))
	commonAncestor, err := chain.FindCommonAncestor(mainItr, forkItr)
	assert.NoError(t, err)
	assert.Equal(t, commonHeadTip, commonAncestor)
}

func TestFindCommonAncestorNoFork(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	head := builder.AppendTo()

	// Add 30 tipsets to the head of the chainStore.
	head = builder.AppendManyTo(30, head)
	headTipSet1 := types.RequireNewTipSet(t, head)
	headIterOne := chain.IterAncestors(ctx, builder, headTipSet1)

	// Now add 19 more tipsets.
	head = builder.AppendManyTo(19, head)
	headTipSet2 := types.RequireNewTipSet(t, head)
	headIterTwo := chain.IterAncestors(ctx, builder, headTipSet2)

	commonAncestor, err := chain.FindCommonAncestor(headIterOne, headIterTwo)
	assert.NoError(t, err)
	assert.Equal(t, headTipSet1, commonAncestor)
}

// This test exercises an edge case fork that our previous common ancestor
// utility handled incorrectly.
func TestFindCommonAncestorNullBlockFork(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	head := builder.AppendTo()

	// Add 10 tipsets to the head of the chainStore.
	commonHead := builder.AppendManyTo(10, head)
	expectedCA := types.RequireNewTipSet(t, commonHead)

	// From the common ancestor, add a block following a null block.
	headAfterNull := builder.BuildTo(commonHead, func(b *chain.BlockBuilder) {
		b.IncHeight(1)
	})
	afterNullTS := types.RequireNewTipSet(t, headAfterNull)
	afterNullItr := chain.IterAncestors(ctx, builder, afterNullTS)

	// Add a block (with no null) on another fork.
	headNoNull := builder.AppendTo(commonHead)
	noNullTS := types.RequireNewTipSet(t, headNoNull)
	noNullItr := chain.IterAncestors(ctx, builder, noNullTS)

	commonAncestor, err := chain.FindCommonAncestor(afterNullItr, noNullItr)
	assert.NoError(t, err)
	assert.Equal(t, expectedCA, commonAncestor)
}
