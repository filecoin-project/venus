package chain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

// Happy path
func TestCollectTipSetsOfHeightAtLeast(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)

	chainLen := 15
	head := builder.AppendManyOn(chainLen, block.UndefTipSet)

	stopHeight := types.NewBlockHeight(uint64(4))
	iterator := chain.IterAncestors(ctx, builder, head)
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
	head := builder.AppendManyOn(chainLen, block.UndefTipSet)

	stopHeight := types.NewBlockHeight(uint64(0))
	iterator := chain.IterAncestors(ctx, builder, head)
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
	head := builder.NewGenesis()

	// Add 30 tipsets to the head of the chainStore.
	head = builder.AppendManyOn(30, head)

	// Now add 10 null blocks and 1 tipset.
	head = builder.BuildOneOn(head, func(b *chain.BlockBuilder) {
		b.IncHeight(10)
	})

	// Now add 19 more tipsets.
	head = builder.AppendManyOn(19, head)

	stopHeight := types.NewBlockHeight(uint64(35))
	iterator := chain.IterAncestors(ctx, builder, head)
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

// Happy path
func TestCollectTipSetsPastHeight(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)

	chainLen := 15
	head := builder.AppendManyOn(chainLen, block.UndefTipSet)

	stopHeight := types.NewBlockHeight(uint64(4))
	iterator := chain.IterAncestors(ctx, builder, head)
	tipsets, err := chain.CollectTipSetsPastHeight(iterator, stopHeight)
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
func TestCollectTipSetsPastHeightZero(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)

	chainLen := 25
	head := builder.AppendManyOn(chainLen, block.UndefTipSet)

	stopHeight := types.NewBlockHeight(uint64(0))
	iterator := chain.IterAncestors(ctx, builder, head)
	tipsets, err := chain.CollectTipSetsPastHeight(iterator, stopHeight)
	assert.NoError(t, err)
	latestHeight, err := tipsets[0].Height()
	require.NoError(t, err)
	assert.Equal(t, uint64(24), latestHeight)
	earliestHeight, err := tipsets[len(tipsets)-1].Height()
	require.NoError(t, err)
	assert.Equal(t, uint64(0), earliestHeight)
	assert.Equal(t, chainLen, len(tipsets))
}

// tipsets at least 1.
func TestCollectTipSetsPastHeightTipsetOne(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)

	chainLen := 25
	head := builder.AppendManyOn(chainLen, block.UndefTipSet)

	stopHeight := types.NewBlockHeight(uint64(25))
	iterator := chain.IterAncestors(ctx, builder, head)
	tipsets, err := chain.CollectTipSetsPastHeight(iterator, stopHeight)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tipsets))
}

// The starting epoch is a null block.
func TestCollectTipSetsPastHeightStartingEpochIsNull(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	head := builder.NewGenesis()

	// Add 30 tipsets to the head of the chainStore.
	head = builder.AppendManyOn(30, head)

	// Now add 10 null blocks and 1 tipset.
	head = builder.BuildOneOn(head, func(b *chain.BlockBuilder) {
		b.IncHeight(10)
	})

	// Now add 19 more tipsets.
	head = builder.AppendManyOn(19, head)

	stopHeight := types.NewBlockHeight(uint64(35))
	iterator := chain.IterAncestors(ctx, builder, head)
	tipsets, err := chain.CollectTipSetsPastHeight(iterator, stopHeight)
	assert.NoError(t, err)
	latestHeight, err := tipsets[0].Height()
	require.NoError(t, err)
	assert.Equal(t, uint64(60), latestHeight)
	earliestHeight, err := tipsets[len(tipsets)-1].Height()
	require.NoError(t, err)
	assert.Equal(t, uint64(30), earliestHeight)
	assert.Equal(t, 21, len(tipsets))
}

func TestCollectAtMostNTipSets(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)

	chainLen := 25
	head := builder.AppendManyOn(chainLen, block.UndefTipSet)

	t.Run("happy path", func(t *testing.T) {
		number := uint(10)
		iterator := chain.IterAncestors(ctx, builder, head)
		tipsets, err := chain.CollectAtMostNTipSets(ctx, iterator, number)
		assert.NoError(t, err)
		assert.Equal(t, 10, len(tipsets))
	})
	t.Run("hit genesis", func(t *testing.T) {
		number := uint(400)
		iterator := chain.IterAncestors(ctx, builder, head)
		tipsets, err := chain.CollectAtMostNTipSets(ctx, iterator, number)
		assert.NoError(t, err)
		assert.Equal(t, 25, len(tipsets))
	})
	t.Run("hit zero", func(t *testing.T) {
		number := uint(0)
		iterator := chain.IterAncestors(ctx, builder, head)
		tipsets, err := chain.CollectAtMostNTipSets(ctx, iterator, number)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(tipsets))
	})
}

// Test the happy path.
// Make a chain of 200 tipsets
// DependentAncestor epochs = 100
func TestGetRecentAncestors(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)

	chainLen := 200
	headBlock := builder.AppendManyBlocksOnBlocks(chainLen)
	head := th.RequireNewTipSet(t, headBlock)

	epochs := uint64(100)
	sampleHeight := types.NewBlockHeight(uint64(headBlock.Height + 1)).Sub(types.NewBlockHeight(epochs))
	ancestors, err := chain.GetRecentAncestors(ctx, head, builder, sampleHeight)
	require.NoError(t, err)
	assert.Equal(t, ancestors[0], head)
	assert.Equal(t, int(epochs), len(ancestors))
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
	head := builder.AppendManyOn(chainLen, block.UndefTipSet)
	h, err := head.Height()
	require.NoError(t, err)
	epochs := uint64(200)

	t.Run("more epochs than chainStore", func(t *testing.T) {
		sampleHeight := types.NewBlockHeight(h + uint64(1)).Sub(types.NewBlockHeight(epochs))
		assert.True(t, sampleHeight.LessThan(types.NewBlockHeight(0)))
		ancestors, err := chain.GetRecentAncestors(ctx, head, builder, sampleHeight)
		require.NoError(t, err)
		assert.Equal(t, chainLen, len(ancestors))
	})
}

// Test case where no block has the start height in the chain due to null blocks.
func TestGetRecentAncestorsStartingEpochIsNull(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	head := builder.NewGenesis()

	// Add 30 tipsets to the head of the chainStore.
	head = builder.AppendManyOn(30, head)
	// Add 10 null blocks and 1 tipset.
	head = builder.BuildOneOn(head, func(b *chain.BlockBuilder) {
		b.IncHeight(10)
	})
	// Add 19 more tipsets, so there are 20 after the nulls.
	len2 := 19
	head = builder.AppendManyOn(len2, head)

	epochs := uint64(22)
	h, err := head.Height()
	require.NoError(t, err)
	sampleHeight := types.NewBlockHeight(h).Sub(types.NewBlockHeight(epochs))
	ancestors, err := chain.GetRecentAncestors(ctx, head, builder, sampleHeight)
	require.NoError(t, err)

	// We expect to see 20+1 tips: the head of the chain plus one after the nulls.
	assert.Equal(t, len2+2, len(ancestors))
	lastBlockHeight, err := ancestors[len(ancestors)-1].Height()
	require.NoError(t, err)
	assert.Equal(t, uint64(30), lastBlockHeight)
}

func TestFindCommonAncestorSameChain(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	head := builder.NewGenesis()
	// Add 30 tipsets to the head of the chainStore.
	head = builder.AppendManyOn(30, head)
	headIterOne := chain.IterAncestors(ctx, builder, head)
	headIterTwo := chain.IterAncestors(ctx, builder, head)
	commonAncestor, err := chain.FindCommonAncestor(headIterOne, headIterTwo)
	assert.NoError(t, err)
	assert.Equal(t, head, commonAncestor)
}

func TestFindCommonAncestorFork(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	head := builder.NewGenesis()

	// Add 3 tipsets to the head of the chainStore.
	commonHeadTip := builder.AppendManyOn(3, head)

	// Grow the fork chain
	lenFork := 10
	forkHead := builder.AppendManyOn(lenFork, commonHeadTip)

	// Grow the main chain
	lenMainChain := 14
	mainHead := builder.AppendManyOn(lenMainChain, commonHeadTip)

	forkItr := chain.IterAncestors(ctx, builder, forkHead)
	mainItr := chain.IterAncestors(ctx, builder, mainHead)
	commonAncestor, err := chain.FindCommonAncestor(mainItr, forkItr)
	assert.NoError(t, err)
	assert.Equal(t, commonHeadTip, commonAncestor)
}

func TestFindCommonAncestorNoFork(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	head := builder.NewGenesis()

	// Add 30 tipsets to the head of the chainStore.
	head = builder.AppendManyOn(30, head)
	headIterOne := chain.IterAncestors(ctx, builder, head)

	// Now add 19 more tipsets.
	expectedAncestor := head
	head = builder.AppendManyOn(19, head)
	headIterTwo := chain.IterAncestors(ctx, builder, head)

	commonAncestor, err := chain.FindCommonAncestor(headIterOne, headIterTwo)
	assert.NoError(t, err)
	assert.True(t, expectedAncestor.Equals(commonAncestor))
}

// This test exercises an edge case fork that our previous common ancestor
// utility handled incorrectly.
func TestFindCommonAncestorNullBlockFork(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	head := builder.NewGenesis()

	// Add 10 tipsets to the head of the chainStore.
	commonHead := builder.AppendManyOn(10, head)

	// From the common ancestor, add a block following a null block.
	headAfterNull := builder.BuildOneOn(commonHead, func(b *chain.BlockBuilder) {
		b.IncHeight(1)
	})
	afterNullItr := chain.IterAncestors(ctx, builder, headAfterNull)

	// Add a block (with no null) on another fork.
	headNoNull := builder.AppendOn(commonHead, 1)
	noNullItr := chain.IterAncestors(ctx, builder, headNoNull)

	commonAncestor, err := chain.FindCommonAncestor(afterNullItr, noNullItr)
	assert.NoError(t, err)
	assert.Equal(t, commonHead, commonAncestor)
}
