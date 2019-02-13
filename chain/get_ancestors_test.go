package chain

import (
	"context"
	"testing"

	"gx/ipfs/QmRXf2uUSdGSunRJsM9wXSUNVwLUGCY3So5fAs7h2CBJVf/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupGetAncestorTests initializes genesis and chain store for tests.
func setupGetAncestorTests(require *require.Assertions) (context.Context, *hamt.CborIpldStore, Store) {
	_, chainStore, cst, _ := initSyncTestDefault(require)
	return context.Background(), cst, chainStore
}

// requireGrowChain grows the given store numBlocks single block tipsets from
// its head.
func requireGrowChain(ctx context.Context, require *require.Assertions, cst *hamt.CborIpldStore, chain Store, numBlocks int) {
	link := chain.Head()
	for i := 0; i < numBlocks; i++ {
		linkBlock := RequireMkFakeChild(require,
			FakeChildParams{Parent: link, GenesisCid: genCid, StateRoot: genStateRoot})
		requirePutBlocks(require, cst, linkBlock)
		link = testhelpers.RequireNewTipSet(require, linkBlock)
		linkTsas := &TipSetAndState{
			TipSet:          link,
			TipSetStateRoot: genStateRoot,
		}
		RequirePutTsas(ctx, require, chain, linkTsas)
	}
	err := chain.SetHead(ctx, link)
	require.NoError(err)
}

// Happy path
func TestCollectTipSetsOfHeightAtLeast(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	ctx, cst, chain := setupGetAncestorTests(require)
	chainLen := 15
	requireGrowChain(ctx, require, cst, chain, chainLen-1)
	ch := chain.BlockHistory(ctx, chain.Head())
	stopHeight := types.NewBlockHeight(uint64(4))
	tipsets, err := CollectTipSetsOfHeightAtLeast(ctx, ch, stopHeight)
	assert.NoError(err)
	latestHeight, err := tipsets[0].Height()
	require.NoError(err)
	assert.Equal(uint64(14), latestHeight)
	earliestHeight, err := tipsets[len(tipsets)-1].Height()
	require.NoError(err)
	assert.Equal(uint64(4), earliestHeight)
	assert.Equal(11, len(tipsets))
}

// Height at least 0.
func TestCollectTipSetsOfHeightAtLeastZero(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	ctx, cst, chain := setupGetAncestorTests(require)
	chainLen := 25
	requireGrowChain(ctx, require, cst, chain, chainLen-1)
	ch := chain.BlockHistory(ctx, chain.Head())
	stopHeight := types.NewBlockHeight(uint64(0))
	tipsets, err := CollectTipSetsOfHeightAtLeast(ctx, ch, stopHeight)
	assert.NoError(err)
	latestHeight, err := tipsets[0].Height()
	require.NoError(err)
	assert.Equal(uint64(24), latestHeight)
	earliestHeight, err := tipsets[len(tipsets)-1].Height()
	require.NoError(err)
	assert.Equal(uint64(0), earliestHeight)
	assert.Equal(25, len(tipsets))
}

// The starting epoch is a null block.
func TestCollectTipSetsOfHeightAtLeastStartingEpochIsNull(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	ctx, cst, chain := setupGetAncestorTests(require)
	// Add 30 tipsets to the head of the chain.
	len1 := 30
	requireGrowChain(ctx, require, cst, chain, len1)

	// Now add 10 null blocks and 1 tipset.
	nullBlocks := uint64(10)
	afterNullBlock := RequireMkFakeChild(require,
		FakeChildParams{Parent: chain.Head(), GenesisCid: genCid, StateRoot: genStateRoot, NullBlockCount: nullBlocks})
	requirePutBlocks(require, cst, afterNullBlock)
	afterNull := testhelpers.RequireNewTipSet(require, afterNullBlock)
	afterNullTsas := &TipSetAndState{
		TipSet:          afterNull,
		TipSetStateRoot: genStateRoot,
	}
	RequirePutTsas(ctx, require, chain, afterNullTsas)
	err := chain.SetHead(ctx, afterNull)
	require.NoError(err)

	// Now add 19 more tipsets.
	len2 := 19
	requireGrowChain(ctx, require, cst, chain, len2)

	ch := chain.BlockHistory(ctx, chain.Head())
	stopHeight := types.NewBlockHeight(uint64(35))
	tipsets, err := CollectTipSetsOfHeightAtLeast(ctx, ch, stopHeight)
	assert.NoError(err)
	latestHeight, err := tipsets[0].Height()
	require.NoError(err)
	assert.Equal(uint64(60), latestHeight)
	earliestHeight, err := tipsets[len(tipsets)-1].Height()
	require.NoError(err)
	assert.Equal(uint64(41), earliestHeight)
	assert.Equal(20, len(tipsets))
}

func TestCollectAtMostNTipSets(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	ctx, cst, chain := setupGetAncestorTests(require)
	chainLen := 25
	requireGrowChain(ctx, require, cst, chain, chainLen-1)
	t.Run("happy path", func(t *testing.T) {
		ch := chain.BlockHistory(ctx, chain.Head())
		number := uint(10)
		tipsets, err := CollectAtMostNTipSets(ctx, ch, number)
		assert.NoError(err)
		assert.Equal(10, len(tipsets))
	})
	t.Run("hit genesis", func(t *testing.T) {
		ch := chain.BlockHistory(ctx, chain.Head())
		number := uint(400)
		tipsets, err := CollectAtMostNTipSets(ctx, ch, number)
		assert.NoError(err)
		assert.Equal(25, len(tipsets))
	})
}

// Test the happy path.
// Make a chain of 200 tipsets
// DependentAncestor epochs = 100
// Lookback = 20
func TestGetRecentAncestors(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	ctx, cst, chain := setupGetAncestorTests(require)
	chainLen := 200
	requireGrowChain(ctx, require, cst, chain, chainLen-1)
	h, err := chain.Head().Height()
	require.NoError(err)
	epochs := uint64(100)
	lookback := uint(20)
	ancestors, err := GetRecentAncestors(ctx, chain.Head(), chain, types.NewBlockHeight(h+uint64(1)), types.NewBlockHeight(epochs), lookback)
	require.NoError(err)

	assert.Equal(ancestors[0], chain.Head())
	assert.Equal(int(epochs)+int(lookback), len(ancestors))
	for i := 0; i < len(ancestors); i++ {
		h, err := ancestors[i].Height()
		assert.NoError(err)
		assert.Equal(h, uint64(chainLen-1-i))
	}
}

// Test case where parameters specify a chain past genesis.
func TestGetRecentAncestorsTruncates(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	ctx, cst, chain := setupGetAncestorTests(require)
	chainLen := 100
	requireGrowChain(ctx, require, cst, chain, chainLen-1)
	h, err := chain.Head().Height()
	require.NoError(err)
	epochs := uint64(200)
	lookback := uint(20)

	t.Run("more epochs than chain", func(t *testing.T) {
		ancestors, err := GetRecentAncestors(ctx, chain.Head(), chain, types.NewBlockHeight(h+uint64(1)), types.NewBlockHeight(epochs), lookback)
		require.NoError(err)
		assert.Equal(chainLen, len(ancestors))
	})

	t.Run("more epochs + lookback than chain", func(t *testing.T) {
		epochs = uint64(60)
		lookback = uint(50)
		ancestors, err := GetRecentAncestors(ctx, chain.Head(), chain, types.NewBlockHeight(h+uint64(1)), types.NewBlockHeight(epochs), lookback)
		require.NoError(err)
		assert.Equal(chainLen, len(ancestors))
	})
}

// Test case where no block has the start height in the chain due to null blocks.
func TestGetRecentAncestorsStartingEpochIsNull(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	ctx, cst, chain := setupGetAncestorTests(require)
	// Add 30 tipsets to the head of the chain.
	len1 := 30
	requireGrowChain(ctx, require, cst, chain, len1)

	// Now add 10 null blocks and 1 tipset.
	nullBlocks := uint64(10)
	afterNullBlock := RequireMkFakeChild(require,
		FakeChildParams{Parent: chain.Head(), GenesisCid: genCid, StateRoot: genStateRoot, NullBlockCount: nullBlocks})
	requirePutBlocks(require, cst, afterNullBlock)
	afterNull := testhelpers.RequireNewTipSet(require, afterNullBlock)
	afterNullTsas := &TipSetAndState{
		TipSet:          afterNull,
		TipSetStateRoot: genStateRoot,
	}
	RequirePutTsas(ctx, require, chain, afterNullTsas)
	err := chain.SetHead(ctx, afterNull)
	require.NoError(err)

	// Now add 19 more tipsets.
	len2 := 19
	requireGrowChain(ctx, require, cst, chain, len2)

	epochs := uint64(28)
	lookback := uint(6)
	h, err := chain.Head().Height()
	require.NoError(err)
	ancestors, err := GetRecentAncestors(ctx, chain.Head(), chain, types.NewBlockHeight(h+uint64(1)), types.NewBlockHeight(epochs), lookback)
	require.NoError(err)

	// We expect to see 20 blocks in the first 28 epochs and an additional 6 for the lookback parameter
	assert.Equal(len2+int(lookback)+1, len(ancestors))
	lastBlockHeight, err := ancestors[len(ancestors)-1].Height()
	require.NoError(err)
	assert.Equal(uint64(25), lastBlockHeight)
}
