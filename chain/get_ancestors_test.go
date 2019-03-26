package chain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/chain"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
)

// setupGetAncestorTests initializes genesis and chain store for tests.
func setupGetAncestorTests(require *require.Assertions) (context.Context, *th.TestFetcher, chain.Store) {
	_, chainStore, _, blockSource := initSyncTestDefault(require)
	return context.Background(), blockSource, chainStore
}

// requireGrowChain grows the given store numBlocks single block tipsets from
// its head.
func requireGrowChain(ctx context.Context, require *require.Assertions, blockSource *th.TestFetcher, chainStore chain.Store, numBlocks int) {
	head := chainStore.GetHead()
	headTipSetAndState, err := chainStore.GetTipSetAndState(ctx, head)
	require.NoError(err)
	link := headTipSetAndState.TipSet

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
		linkBlock := th.RequireMkFakeChild(require, fakeChildParams)
		requirePutBlocks(require, blockSource, linkBlock)
		link = th.RequireNewTipSet(require, linkBlock)
		linkTsas := &chain.TipSetAndState{
			TipSet:          link,
			TipSetStateRoot: genStateRoot,
		}
		th.RequirePutTsas(ctx, require, chainStore, linkTsas)
	}
	err = chainStore.SetHead(ctx, link)
	require.NoError(err)
}

// Happy path
func TestCollectTipSetsOfHeightAtLeast(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	ctx, blockSource, chainStore := setupGetAncestorTests(require)
	chainLen := 15
	requireGrowChain(ctx, require, blockSource, chainStore, chainLen-1)
	head := chainStore.GetHead()
	headTipSetAndState, err := chainStore.GetTipSetAndState(ctx, head)
	require.NoError(err)
	ch := chainStore.BlockHistory(ctx, headTipSetAndState.TipSet)
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
	ctx, blockSource, chainStore := setupGetAncestorTests(require)
	chainLen := 25
	requireGrowChain(ctx, require, blockSource, chainStore, chainLen-1)
	head := chainStore.GetHead()
	headTipSetAndState, err := chainStore.GetTipSetAndState(ctx, head)
	require.NoError(err)
	ch := chainStore.BlockHistory(ctx, headTipSetAndState.TipSet)
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
	ctx, blockSource, chainStore := setupGetAncestorTests(require)
	// Add 30 tipsets to the head of the chainStore.
	len1 := 30
	requireGrowChain(ctx, require, blockSource, chainStore, len1)

	// Now add 10 null blocks and 1 tipset.

	signer, ki := types.NewMockSignersAndKeyInfo(1)
	mockSignerPubKey := ki[0].PublicKey()

	nullBlocks := uint64(10)

	head := chainStore.GetHead()
	headTipSetAndState, err := chainStore.GetTipSetAndState(ctx, head)
	require.NoError(err)
	fakeChildParams := th.FakeChildParams{
		Parent:         headTipSetAndState.TipSet,
		GenesisCid:     genCid,
		NullBlockCount: nullBlocks,
		Signer:         signer,
		MinerPubKey:    mockSignerPubKey,
		StateRoot:      genStateRoot,
	}

	afterNullBlock := th.RequireMkFakeChild(require, fakeChildParams)
	requirePutBlocks(require, blockSource, afterNullBlock)
	afterNull := th.RequireNewTipSet(require, afterNullBlock)
	afterNullTsas := &chain.TipSetAndState{
		TipSet:          afterNull,
		TipSetStateRoot: genStateRoot,
	}
	th.RequirePutTsas(ctx, require, chainStore, afterNullTsas)
	err = chainStore.SetHead(ctx, afterNull)
	require.NoError(err)

	// Now add 19 more tipsets.
	len2 := 19
	requireGrowChain(ctx, require, blockSource, chainStore, len2)

	head = chainStore.GetHead()
	headTipSetAndState, err = chainStore.GetTipSetAndState(ctx, head)
	require.NoError(err)
	ch := chainStore.BlockHistory(ctx, headTipSetAndState.TipSet)
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
	ctx, blockSource, chainStore := setupGetAncestorTests(require)
	chainLen := 25
	requireGrowChain(ctx, require, blockSource, chainStore, chainLen-1)
	t.Run("happy path", func(t *testing.T) {
		head := chainStore.GetHead()
		headTipSetAndState, err := chainStore.GetTipSetAndState(ctx, head)
		require.NoError(err)
		ch := chainStore.BlockHistory(ctx, headTipSetAndState.TipSet)
		number := uint(10)
		tipsets, err := chain.CollectAtMostNTipSets(ctx, ch, number)
		assert.NoError(err)
		assert.Equal(10, len(tipsets))
	})
	t.Run("hit genesis", func(t *testing.T) {
		head := chainStore.GetHead()
		headTipSetAndState, err := chainStore.GetTipSetAndState(ctx, head)
		require.NoError(err)
		ch := chainStore.BlockHistory(ctx, headTipSetAndState.TipSet)
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
	ctx, blockSource, chainStore := setupGetAncestorTests(require)
	chainLen := 200
	requireGrowChain(ctx, require, blockSource, chainStore, chainLen-1)
	head := chainStore.GetHead()
	headTipSetAndState, err := chainStore.GetTipSetAndState(ctx, head)
	require.NoError(err)
	h, err := headTipSetAndState.TipSet.Height()
	require.NoError(err)
	epochs := uint64(100)
	lookback := uint(20)
	ancestors, err := chain.GetRecentAncestors(ctx, headTipSetAndState.TipSet, chainStore, types.NewBlockHeight(h+uint64(1)), types.NewBlockHeight(epochs), lookback)
	require.NoError(err)
	assert.Equal(ancestors[0], headTipSetAndState.TipSet)
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
	ctx, blockSource, chainStore := setupGetAncestorTests(require)
	chainLen := 100
	requireGrowChain(ctx, require, blockSource, chainStore, chainLen-1)
	head := chainStore.GetHead()
	headTipSetAndState, err := chainStore.GetTipSetAndState(ctx, head)
	require.NoError(err)
	h, err := headTipSetAndState.TipSet.Height()
	require.NoError(err)
	epochs := uint64(200)
	lookback := uint(20)

	t.Run("more epochs than chainStore", func(t *testing.T) {
		ancestors, err := chain.GetRecentAncestors(ctx, headTipSetAndState.TipSet, chainStore, types.NewBlockHeight(h+uint64(1)), types.NewBlockHeight(epochs), lookback)
		require.NoError(err)
		assert.Equal(chainLen, len(ancestors))
	})

	t.Run("more epochs + lookback than chainStore", func(t *testing.T) {
		epochs = uint64(60)
		lookback = uint(50)
		ancestors, err := chain.GetRecentAncestors(ctx, headTipSetAndState.TipSet, chainStore, types.NewBlockHeight(h+uint64(1)), types.NewBlockHeight(epochs), lookback)
		require.NoError(err)
		assert.Equal(chainLen, len(ancestors))
	})
}

// Test case where no block has the start height in the chain due to null blocks.
func TestGetRecentAncestorsStartingEpochIsNull(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	ctx, blockSource, chainStore := setupGetAncestorTests(require)
	// Add 30 tipsets to the head of the chainStore.
	len1 := 30
	requireGrowChain(ctx, require, blockSource, chainStore, len1)

	// Now add 10 null blocks and 1 tipset.
	signer, ki := types.NewMockSignersAndKeyInfo(1)
	mockSignerPubKey := ki[0].PublicKey()

	nullBlocks := uint64(10)

	head := chainStore.GetHead()
	headTipSetAndState, err := chainStore.GetTipSetAndState(ctx, head)
	require.NoError(err)
	fakeChildParams := th.FakeChildParams{
		Parent:         headTipSetAndState.TipSet,
		GenesisCid:     genCid,
		StateRoot:      genStateRoot,
		NullBlockCount: nullBlocks,
		Signer:         signer,
		MinerPubKey:    mockSignerPubKey,
	}
	afterNullBlock := th.RequireMkFakeChild(require, fakeChildParams)
	requirePutBlocks(require, blockSource, afterNullBlock)
	afterNull := th.RequireNewTipSet(require, afterNullBlock)
	afterNullTsas := &chain.TipSetAndState{
		TipSet:          afterNull,
		TipSetStateRoot: genStateRoot,
	}
	th.RequirePutTsas(ctx, require, chainStore, afterNullTsas)
	err = chainStore.SetHead(ctx, afterNull)
	require.NoError(err)

	// Now add 19 more tipsets.
	len2 := 19
	requireGrowChain(ctx, require, blockSource, chainStore, len2)

	epochs := uint64(28)
	lookback := uint(6)
	head = chainStore.GetHead()
	headTipSetAndState, err = chainStore.GetTipSetAndState(ctx, head)
	require.NoError(err)
	h, err := headTipSetAndState.TipSet.Height()
	require.NoError(err)
	ancestors, err := chain.GetRecentAncestors(ctx, headTipSetAndState.TipSet, chainStore, types.NewBlockHeight(h+uint64(1)), types.NewBlockHeight(epochs), lookback)
	require.NoError(err)

	// We expect to see 20 blocks in the first 28 epochs and an additional 6 for the lookback parameter
	assert.Equal(len2+int(lookback)+1, len(ancestors))
	lastBlockHeight, err := ancestors[len(ancestors)-1].Height()
	require.NoError(err)
	assert.Equal(uint64(25), lastBlockHeight)
}
