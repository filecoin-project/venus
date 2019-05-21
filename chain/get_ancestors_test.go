package chain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/chain"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

// setupGetAncestorTests initializes genesis and chain store for tests.
func setupGetAncestorTests(t *testing.T) (context.Context, *th.TestFetcher, chain.Store) {
	_, chainStore, _, blockSource := initSyncTestDefault(t)
	return context.Background(), blockSource, chainStore
}

// requireGrowChain grows the given store numBlocks single block tipsets from
// its head.
func requireGrowChain(ctx context.Context, t *testing.T, blockSource *th.TestFetcher, chainStore chain.Store, numBlocks int) {
	link := requireHeadTipset(t, chainStore)

	signer, ki := types.NewMockSignersAndKeyInfo(1)
	mockSignerPubKey := ki[0].PublicKey()

	for i := 0; i < numBlocks; i++ {
		fakeChildParams := th.FakeChildParams{
			Parent:      link,
			GenesisCid:  genCid,
			Signer:      signer,
			MinerPubKey: mockSignerPubKey,
			StateRoot:   genStateRoot,
		}
		linkBlock := th.RequireMkFakeChild(t, fakeChildParams)
		requirePutBlocks(t, blockSource, linkBlock)
		link = th.RequireNewTipSet(t, linkBlock)
		linkTsas := &chain.TipSetAndState{
			TipSet:          link,
			TipSetStateRoot: genStateRoot,
		}
		th.RequirePutTsas(ctx, t, chainStore, linkTsas)
	}
	err := chainStore.SetHead(ctx, link)
	require.NoError(t, err)
}

// Happy path
func TestCollectTipSetsOfHeightAtLeast(t *testing.T) {
	tf.BadUnitTestWithSideEffects(t)

	ctx, blockSource, chainStore := setupGetAncestorTests(t)
	chainLen := 15
	requireGrowChain(ctx, t, blockSource, chainStore, chainLen-1)
	stopHeight := types.NewBlockHeight(uint64(4))
	iterator := chain.IterAncestors(ctx, chainStore, requireHeadTipset(t, chainStore))
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
	tf.BadUnitTestWithSideEffects(t)

	ctx, blockSource, chainStore := setupGetAncestorTests(t)
	chainLen := 25
	requireGrowChain(ctx, t, blockSource, chainStore, chainLen-1)
	stopHeight := types.NewBlockHeight(uint64(0))
	iterator := chain.IterAncestors(ctx, chainStore, requireHeadTipset(t, chainStore))
	tipsets, err := chain.CollectTipSetsOfHeightAtLeast(ctx, iterator, stopHeight)
	assert.NoError(t, err)
	latestHeight, err := tipsets[0].Height()
	require.NoError(t, err)
	assert.Equal(t, uint64(24), latestHeight)
	earliestHeight, err := tipsets[len(tipsets)-1].Height()
	require.NoError(t, err)
	assert.Equal(t, uint64(0), earliestHeight)
	assert.Equal(t, 25, len(tipsets))
}

// The starting epoch is a null block.
func TestCollectTipSetsOfHeightAtLeastStartingEpochIsNull(t *testing.T) {
	tf.BadUnitTestWithSideEffects(t)

	ctx, blockSource, chainStore := setupGetAncestorTests(t)
	// Add 30 tipsets to the head of the chainStore.
	len1 := 30
	requireGrowChain(ctx, t, blockSource, chainStore, len1)

	// Now add 10 null blocks and 1 tipset.

	signer, ki := types.NewMockSignersAndKeyInfo(1)
	mockSignerPubKey := ki[0].PublicKey()

	nullBlocks := uint64(10)

	fakeChildParams := th.FakeChildParams{
		Parent:         requireHeadTipset(t, chainStore),
		GenesisCid:     genCid,
		NullBlockCount: nullBlocks,
		Signer:         signer,
		MinerPubKey:    mockSignerPubKey,
		StateRoot:      genStateRoot,
	}

	afterNullBlock := th.RequireMkFakeChild(t, fakeChildParams)
	requirePutBlocks(t, blockSource, afterNullBlock)
	afterNull := th.RequireNewTipSet(t, afterNullBlock)
	afterNullTsas := &chain.TipSetAndState{
		TipSet:          afterNull,
		TipSetStateRoot: genStateRoot,
	}
	th.RequirePutTsas(ctx, t, chainStore, afterNullTsas)
	err := chainStore.SetHead(ctx, afterNull)
	require.NoError(t, err)

	// Now add 19 more tipsets.
	len2 := 19
	requireGrowChain(ctx, t, blockSource, chainStore, len2)

	stopHeight := types.NewBlockHeight(uint64(35))
	iterator := chain.IterAncestors(ctx, chainStore, requireHeadTipset(t, chainStore))
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
	tf.BadUnitTestWithSideEffects(t)

	ctx, blockSource, chainStore := setupGetAncestorTests(t)
	chainLen := 25
	requireGrowChain(ctx, t, blockSource, chainStore, chainLen-1)
	t.Run("happy path", func(t *testing.T) {
		number := uint(10)
		iterator := chain.IterAncestors(ctx, chainStore, requireHeadTipset(t, chainStore))
		tipsets, err := chain.CollectAtMostNTipSets(ctx, iterator, number)
		assert.NoError(t, err)
		assert.Equal(t, 10, len(tipsets))
	})
	t.Run("hit genesis", func(t *testing.T) {
		number := uint(400)
		iterator := chain.IterAncestors(ctx, chainStore, requireHeadTipset(t, chainStore))
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
	tf.BadUnitTestWithSideEffects(t)

	ctx, blockSource, chainStore := setupGetAncestorTests(t)
	chainLen := 200
	requireGrowChain(ctx, t, blockSource, chainStore, chainLen-1)
	head := requireHeadTipset(t, chainStore)
	h, err := head.Height()
	require.NoError(t, err)
	epochs := uint64(100)
	lookback := uint(20)
	ancestors, err := chain.GetRecentAncestors(ctx, head, chainStore, types.NewBlockHeight(h+uint64(1)), types.NewBlockHeight(epochs), lookback)
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
	tf.BadUnitTestWithSideEffects(t)

	ctx, blockSource, chainStore := setupGetAncestorTests(t)
	chainLen := 100
	requireGrowChain(ctx, t, blockSource, chainStore, chainLen-1)
	h, err := requireHeadTipset(t, chainStore).Height()
	require.NoError(t, err)
	epochs := uint64(200)
	lookback := uint(20)

	t.Run("more epochs than chainStore", func(t *testing.T) {
		ancestors, err := chain.GetRecentAncestors(ctx, requireHeadTipset(t, chainStore), chainStore, types.NewBlockHeight(h+uint64(1)), types.NewBlockHeight(epochs), lookback)
		require.NoError(t, err)
		assert.Equal(t, chainLen, len(ancestors))
	})

	t.Run("more epochs + lookback than chainStore", func(t *testing.T) {
		epochs = uint64(60)
		lookback = uint(50)
		ancestors, err := chain.GetRecentAncestors(ctx, requireHeadTipset(t, chainStore), chainStore, types.NewBlockHeight(h+uint64(1)), types.NewBlockHeight(epochs), lookback)
		require.NoError(t, err)
		assert.Equal(t, chainLen, len(ancestors))
	})
}

// Test case where no block has the start height in the chain due to null blocks.
func TestGetRecentAncestorsStartingEpochIsNull(t *testing.T) {
	tf.BadUnitTestWithSideEffects(t)

	ctx, blockSource, chainStore := setupGetAncestorTests(t)
	// Add 30 tipsets to the head of the chainStore.
	len1 := 30
	requireGrowChain(ctx, t, blockSource, chainStore, len1)

	// Now add 10 null blocks and 1 tipset.
	signer, ki := types.NewMockSignersAndKeyInfo(1)
	mockSignerPubKey := ki[0].PublicKey()

	nullBlocks := uint64(10)

	fakeChildParams := th.FakeChildParams{
		Parent:         requireHeadTipset(t, chainStore),
		GenesisCid:     genCid,
		StateRoot:      genStateRoot,
		NullBlockCount: nullBlocks,
		Signer:         signer,
		MinerPubKey:    mockSignerPubKey,
	}
	afterNullBlock := th.RequireMkFakeChild(t, fakeChildParams)
	requirePutBlocks(t, blockSource, afterNullBlock)
	afterNull := th.RequireNewTipSet(t, afterNullBlock)
	afterNullTsas := &chain.TipSetAndState{
		TipSet:          afterNull,
		TipSetStateRoot: genStateRoot,
	}
	th.RequirePutTsas(ctx, t, chainStore, afterNullTsas)
	err := chainStore.SetHead(ctx, afterNull)
	require.NoError(t, err)

	// Now add 19 more tipsets.
	len2 := 19
	requireGrowChain(ctx, t, blockSource, chainStore, len2)

	epochs := uint64(28)
	lookback := uint(6)
	headTipSet := requireHeadTipset(t, chainStore)
	h, err := headTipSet.Height()
	require.NoError(t, err)
	ancestors, err := chain.GetRecentAncestors(ctx, headTipSet, chainStore, types.NewBlockHeight(h+uint64(1)), types.NewBlockHeight(epochs), lookback)
	require.NoError(t, err)

	// We expect to see 20 blocks in the first 28 epochs and an additional 6 for the lookback parameter
	assert.Equal(t, len2+int(lookback)+1, len(ancestors))
	lastBlockHeight, err := ancestors[len(ancestors)-1].Height()
	require.NoError(t, err)
	assert.Equal(t, uint64(25), lastBlockHeight)
}

func TestFindCommonAncestorSameChain(t *testing.T) {
	tf.BadUnitTestWithSideEffects(t)
	ctx, blockSource, chainStore := setupGetAncestorTests(t)
	// Add 30 tipsets to the head of the chainStore.
	len1 := 30
	requireGrowChain(ctx, t, blockSource, chainStore, len1)
	headTipSet := requireHeadTipset(t, chainStore)
	headIterOne := chain.IterAncestors(ctx, chainStore, headTipSet)
	headIterTwo := chain.IterAncestors(ctx, chainStore, headTipSet)
	commonAncestor, err := chain.FindCommonAncestor(headIterOne, headIterTwo)
	assert.NoError(t, err)
	assert.Equal(t, headTipSet, commonAncestor)
}

func TestFindCommonAncestorFork(t *testing.T) {

	tf.BadUnitTestWithSideEffects(t)
	ctx, blockSource, chainStore := setupGetAncestorTests(t)
	// Add 3 tipsets to the head of the chainStore.
	len1 := 3
	requireGrowChain(ctx, t, blockSource, chainStore, len1)
	headTipSetCA := requireHeadTipset(t, chainStore)

	// make the first fork tipset
	signer, ki := types.NewMockSignersAndKeyInfo(1)
	mockSignerPubKey := ki[0].PublicKey()
	fakeChildParams := th.FakeChildParams{
		Parent:      headTipSetCA,
		GenesisCid:  genCid,
		Signer:      signer,
		MinerPubKey: mockSignerPubKey,
		StateRoot:   genStateRoot,
		Nonce:       uint64(4),
	}

	firstForkBlock := th.RequireMkFakeChild(t, fakeChildParams)
	requirePutBlocks(t, blockSource, firstForkBlock)
	firstForkTS := th.RequireNewTipSet(t, firstForkBlock)
	firstForkTsas := &chain.TipSetAndState{
		TipSet:          firstForkTS,
		TipSetStateRoot: genStateRoot,
	}
	th.RequirePutTsas(ctx, t, chainStore, firstForkTsas)
	err := chainStore.SetHead(ctx, firstForkTS)
	require.NoError(t, err)

	// grow the fork by 10 blocks
	lenFork := 10
	requireGrowChain(ctx, t, blockSource, chainStore, lenFork)
	headTipSetFork := requireHeadTipset(t, chainStore)
	headIterFork := chain.IterAncestors(ctx, chainStore, headTipSetFork)

	// go back and complete the original chain
	err = chainStore.SetHead(ctx, headTipSetCA)
	require.NoError(t, err)
	lenMainChain := 14
	requireGrowChain(ctx, t, blockSource, chainStore, lenMainChain)
	headTipSetMainChain := requireHeadTipset(t, chainStore)
	headIterMainChain := chain.IterAncestors(ctx, chainStore, headTipSetMainChain)

	commonAncestor, err := chain.FindCommonAncestor(headIterMainChain, headIterFork)
	assert.NoError(t, err)
	assert.Equal(t, headTipSetCA, commonAncestor)
}

func TestFindCommonAncestorNoFork(t *testing.T) {
	tf.BadUnitTestWithSideEffects(t)
	ctx, blockSource, chainStore := setupGetAncestorTests(t)
	// Add 30 tipsets to the head of the chainStore.
	len1 := 30
	requireGrowChain(ctx, t, blockSource, chainStore, len1)
	headTipSet1 := requireHeadTipset(t, chainStore)
	headIterOne := chain.IterAncestors(ctx, chainStore, headTipSet1)
	// Now add 19 more tipsets.
	len2 := 19
	requireGrowChain(ctx, t, blockSource, chainStore, len2)
	headTipSet2 := requireHeadTipset(t, chainStore)
	headIterTwo := chain.IterAncestors(ctx, chainStore, headTipSet2)
	commonAncestor, err := chain.FindCommonAncestor(headIterOne, headIterTwo)
	assert.NoError(t, err)
	assert.Equal(t, headTipSet1, commonAncestor)
}

// This test exercises an edge case fork that our previous common ancestor
// utility handled incorrectly.
func TestFindCommonAncestorNullBlockFork(t *testing.T) {
	tf.BadUnitTestWithSideEffects(t)
	ctx, blockSource, chainStore := setupGetAncestorTests(t)
	// Add 10 tipsets to the head of the chainStore.
	len1 := 10
	requireGrowChain(ctx, t, blockSource, chainStore, len1)
	expectedCA := requireHeadTipset(t, chainStore)

	// add a null block and another block to the head
	signer, ki := types.NewMockSignersAndKeyInfo(1)
	mockSignerPubKey := ki[0].PublicKey()
	fakeChildParams := th.FakeChildParams{
		Parent:         expectedCA,
		GenesisCid:     genCid,
		Signer:         signer,
		MinerPubKey:    mockSignerPubKey,
		StateRoot:      genStateRoot,
		NullBlockCount: uint64(1),
	}

	afterNullBlock := th.RequireMkFakeChild(t, fakeChildParams)
	requirePutBlocks(t, blockSource, afterNullBlock)
	afterNullTS := th.RequireNewTipSet(t, afterNullBlock)
	afterNullTsas := &chain.TipSetAndState{
		TipSet:          afterNullTS,
		TipSetStateRoot: genStateRoot,
	}
	th.RequirePutTsas(ctx, t, chainStore, afterNullTsas)
	afterNullIter := chain.IterAncestors(ctx, chainStore, afterNullTS)

	// grow the fork by 1 block on the other fork
	len2 := 1
	requireGrowChain(ctx, t, blockSource, chainStore, len2)
	mainChainTS := requireHeadTipset(t, chainStore)
	mainChainIter := chain.IterAncestors(ctx, chainStore, mainChainTS)

	commonAncestor, err := chain.FindCommonAncestor(afterNullIter, mainChainIter)
	assert.NoError(t, err)
	assert.Equal(t, expectedCA, commonAncestor)
}
