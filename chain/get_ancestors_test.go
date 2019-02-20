package chain_test

import (
	"context"
	"github.com/filecoin-project/go-filecoin/chain"
	"testing"

	"gx/ipfs/QmNf3wujpV2Y7Lnj2hy2UrmuX8bhMDStRHbnSLh7Ypf36h/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

// setupGetAncestorTests initializes genesis and chain store for tests.
func setupGetAncestorTests(require *require.Assertions) (context.Context, *hamt.CborIpldStore, chain.Store) {
	_, chainStore, cst, _ := initSyncTestDefault(require)
	return context.Background(), cst, chainStore
}

// requireGrowChain grows the given store numBlocks single block tipsets from
// its head.
func requireGrowChain(ctx context.Context, require *require.Assertions, cst *hamt.CborIpldStore, chainStore chain.Store, numBlocks int) {
	link := chainStore.Head()
	for i := 0; i < numBlocks; i++ {
		linkBlock := chain.RequireMkFakeChild(require,
			chain.FakeChildParams{Parent: link, GenesisCid: genCid, StateRoot: genStateRoot})
		requirePutBlocks(require, cst, linkBlock)
		link = testhelpers.RequireNewTipSet(require, linkBlock)
		linkTsas := &chain.TipSetAndState{
			TipSet:          link,
			TipSetStateRoot: genStateRoot,
		}
		chain.RequirePutTsas(ctx, require, chainStore, linkTsas)
	}
	err := chainStore.SetHead(ctx, link)
	require.NoError(err)
}

// Happy path
func TestCollectTipSetsOfHeightAtLeast(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	ctx, cst, chainStore := setupGetAncestorTests(require)
	chainLen := 15
	requireGrowChain(ctx, require, cst, chainStore, chainLen-1)
	ch := chainStore.BlockHistory(ctx, chainStore.Head())
	stopHeight := types.NewBlockHeight(uint64(4))
	tipsets, err := chain.CollectTipSetsOfHeightAtLeast(ctx, ch, stopHeight)
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
	ctx, cst, chainStore := setupGetAncestorTests(require)
	chainLen := 25
	requireGrowChain(ctx, require, cst, chainStore, chainLen-1)
	ch := chainStore.BlockHistory(ctx, chainStore.Head())
	stopHeight := types.NewBlockHeight(uint64(0))
	tipsets, err := chain.CollectTipSetsOfHeightAtLeast(ctx, ch, stopHeight)
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
	ctx, cst, chainStore := setupGetAncestorTests(require)
	// Add 30 tipsets to the head of the chainStore.
	len1 := 30
	requireGrowChain(ctx, require, cst, chainStore, len1)

	// Now add 10 null blocks and 1 tipset.
	nullBlocks := uint64(10)
	afterNullBlock := chain.RequireMkFakeChild(require,
		chain.FakeChildParams{Parent: chainStore.Head(), GenesisCid: genCid, StateRoot: genStateRoot, NullBlockCount: nullBlocks})
	requirePutBlocks(require, cst, afterNullBlock)
	afterNull := testhelpers.RequireNewTipSet(require, afterNullBlock)
	afterNullTsas := &chain.TipSetAndState{
		TipSet:          afterNull,
		TipSetStateRoot: genStateRoot,
	}
	chain.RequirePutTsas(ctx, require, chainStore, afterNullTsas)
	err := chainStore.SetHead(ctx, afterNull)
	require.NoError(err)

	// Now add 19 more tipsets.
	len2 := 19
	requireGrowChain(ctx, require, cst, chainStore, len2)

	ch := chainStore.BlockHistory(ctx, chainStore.Head())
	stopHeight := types.NewBlockHeight(uint64(35))
	tipsets, err := chain.CollectTipSetsOfHeightAtLeast(ctx, ch, stopHeight)
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
	ctx, cst, chainStore := setupGetAncestorTests(require)
	chainLen := 25
	requireGrowChain(ctx, require, cst, chainStore, chainLen-1)
	t.Run("happy path", func(t *testing.T) {
		ch := chainStore.BlockHistory(ctx, chainStore.Head())
		number := uint(10)
		tipsets, err := chain.CollectAtMostNTipSets(ctx, ch, number)
		assert.NoError(err)
		assert.Equal(10, len(tipsets))
	})
	t.Run("hit genesis", func(t *testing.T) {
		ch := chainStore.BlockHistory(ctx, chainStore.Head())
		number := uint(400)
		tipsets, err := chain.CollectAtMostNTipSets(ctx, ch, number)
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
	ctx, cst, chainStore := setupGetAncestorTests(require)
	chainLen := 200
	requireGrowChain(ctx, require, cst, chainStore, chainLen-1)
	h, err := chainStore.Head().Height()
	require.NoError(err)
	epochs := uint64(100)
	lookback := uint(20)
	ancestors, err := chain.GetRecentAncestors(ctx, chainStore.Head(), chainStore, types.NewBlockHeight(h+uint64(1)), types.NewBlockHeight(epochs), lookback)
	require.NoError(err)

	assert.Equal(ancestors[0], chainStore.Head())
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
	ctx, cst, chainStore := setupGetAncestorTests(require)
	chainLen := 100
	requireGrowChain(ctx, require, cst, chainStore, chainLen-1)
	h, err := chainStore.Head().Height()
	require.NoError(err)
	epochs := uint64(200)
	lookback := uint(20)

	t.Run("more epochs than chainStore", func(t *testing.T) {
		ancestors, err := chain.GetRecentAncestors(ctx, chainStore.Head(), chainStore, types.NewBlockHeight(h+uint64(1)), types.NewBlockHeight(epochs), lookback)
		require.NoError(err)
		assert.Equal(chainLen, len(ancestors))
	})

	t.Run("more epochs + lookback than chainStore", func(t *testing.T) {
		epochs = uint64(60)
		lookback = uint(50)
		ancestors, err := chain.GetRecentAncestors(ctx, chainStore.Head(), chainStore, types.NewBlockHeight(h+uint64(1)), types.NewBlockHeight(epochs), lookback)
		require.NoError(err)
		assert.Equal(chainLen, len(ancestors))
	})
}

// Test case where no block has the start height in the chain due to null blocks.
func TestGetRecentAncestorsStartingEpochIsNull(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	ctx, cst, chainStore := setupGetAncestorTests(require)
	// Add 30 tipsets to the head of the chainStore.
	len1 := 30
	requireGrowChain(ctx, require, cst, chainStore, len1)

	// Now add 10 null blocks and 1 tipset.
	nullBlocks := uint64(10)
	afterNullBlock := chain.RequireMkFakeChild(require,
		chain.FakeChildParams{Parent: chainStore.Head(), GenesisCid: genCid, StateRoot: genStateRoot, NullBlockCount: nullBlocks})
	requirePutBlocks(require, cst, afterNullBlock)
	afterNull := testhelpers.RequireNewTipSet(require, afterNullBlock)
	afterNullTsas := &chain.TipSetAndState{
		TipSet:          afterNull,
		TipSetStateRoot: genStateRoot,
	}
	chain.RequirePutTsas(ctx, require, chainStore, afterNullTsas)
	err := chainStore.SetHead(ctx, afterNull)
	require.NoError(err)

	// Now add 19 more tipsets.
	len2 := 19
	requireGrowChain(ctx, require, cst, chainStore, len2)

	epochs := uint64(28)
	lookback := uint(6)
	h, err := chainStore.Head().Height()
	require.NoError(err)
	ancestors, err := chain.GetRecentAncestors(ctx, chainStore.Head(), chainStore, types.NewBlockHeight(h+uint64(1)), types.NewBlockHeight(epochs), lookback)
	require.NoError(err)

	// We expect to see 20 blocks in the first 28 epochs and an additional 6 for the lookback parameter
	assert.Equal(len2+int(lookback)+1, len(ancestors))
	lastBlockHeight, err := ancestors[len(ancestors)-1].Height()
	require.NoError(err)
	assert.Equal(uint64(25), lastBlockHeight)
}
