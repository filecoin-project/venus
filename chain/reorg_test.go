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

func TestIsReorgFork(t *testing.T) {
	tf.UnitTest(t)
	dstP := initDSTParams()
	ctx, blockSource, chainStore := setupGetAncestorTests(t, dstP)
	// main chain has 3 blocks past CA, fork has 1
	old, new, common := getForkOldNewCommon(ctx, t, chainStore, blockSource, dstP, 2, 3, 1)
	assert.True(t, chain.IsReorg(old, new, common))
}
func TestIsReorgPrefix(t *testing.T) {
	tf.UnitTest(t)
	dstP := initDSTParams()
	ctx, blockSource, chainStore := setupGetAncestorTests(t, dstP)
	// Old head is a direct ancestor of new head
	old, new, common := getForkOldNewCommon(ctx, t, chainStore, blockSource, dstP, 2, 3, 0)
	assert.False(t, chain.IsReorg(old, new, common))
}

func TestIsReorgSubset(t *testing.T) {
	tf.UnitTest(t)
	dstP := initDSTParams()
	ctx, blockSource, chainStore := setupGetAncestorTests(t, dstP)
	old, new, common := getSubsetOldNewCommon(ctx, t, chainStore, blockSource, dstP, 2)
	assert.False(t, chain.IsReorg(old, new, common))
}

func TestReorgDiffFork(t *testing.T) {
	tf.UnitTest(t)
	dstP := initDSTParams()
	ctx, blockSource, chainStore := setupGetAncestorTests(t, dstP)
	// main chain has 11 blocks past CA, fork has 10
	old, new, common := getForkOldNewCommon(ctx, t, chainStore, blockSource, dstP, 10, 11, 10)

	dropped, added, err := chain.ReorgDiff(old, new, common)
	assert.NoError(t, err)
	assert.Equal(t, uint64(10), dropped)
	assert.Equal(t, uint64(11), added)
}

func TestReorgDiffSubset(t *testing.T) {
	tf.UnitTest(t)
	dstP := initDSTParams()
	ctx, blockSource, chainStore := setupGetAncestorTests(t, dstP)
	old, new, common := getSubsetOldNewCommon(ctx, t, chainStore, blockSource, dstP, 10)

	dropped, added, err := chain.ReorgDiff(old, new, common)
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), dropped)
	assert.Equal(t, uint64(1), added)
}

// getForkOldNewCommon is a testing helper function that creates and stores a
// blockchain in the chainStore.  The blockchain forks and the common ancestor
// block is 'a' blocks after the genesis block.  The  main chain has an
// additional 'b' blocks, the fork has an additional 'c' blocks.  This function
// returns the forked head, the main head and the common ancestor
func getForkOldNewCommon(ctx context.Context, t *testing.T, chainStore *chain.Store, blockSource *th.TestFetcher, dstP *DefaultSyncerTestParams, a, b, c uint) (types.TipSet, types.TipSet, types.TipSet) {
	// Add a - 1 tipsets to the head of the chainStore.
	requireGrowChain(ctx, t, blockSource, chainStore, a, dstP)
	commonAncestor := requireHeadTipset(t, chainStore)

	if c > 0 {
		// make the first fork tipset (need to do manually to set nonce)
		signer, ki := types.NewMockSignersAndKeyInfo(1)
		mockSignerPubKey := ki[0].PublicKey()
		fakeChildParams := th.FakeChildParams{
			Parent:      commonAncestor,
			GenesisCid:  dstP.genCid,
			Signer:      signer,
			MinerPubKey: mockSignerPubKey,
			StateRoot:   dstP.genStateRoot,
			Nonce:       uint64(1),
		}

		firstForkBlock := th.RequireMkFakeChild(t, fakeChildParams)
		requirePutBlocks(t, blockSource, firstForkBlock)
		firstForkTs := th.RequireNewTipSet(t, firstForkBlock)
		firstForkTsas := &chain.TipSetAndState{
			TipSet:          firstForkTs,
			TipSetStateRoot: dstP.genStateRoot,
		}
		require.NoError(t, chainStore.PutTipSetAndState(ctx, firstForkTsas))
		err := chainStore.SetHead(ctx, firstForkTs)
		require.NoError(t, err)

		// grow the fork by (c - 1) blocks (c total)
		requireGrowChain(ctx, t, blockSource, chainStore, c-1, dstP)
	}

	oldHead := requireHeadTipset(t, chainStore)

	// go back and complete the original chain
	err := chainStore.SetHead(ctx, commonAncestor)
	require.NoError(t, err)
	requireGrowChain(ctx, t, blockSource, chainStore, b, dstP)
	newHead := requireHeadTipset(t, chainStore)

	return oldHead, newHead, commonAncestor
}

// getSubsetOldNewCommon is a testing helper function that creates and stores
// a blockchain in the chainStore.  The blockchain has 'a' blocks after genesis
// and then a fork.  The forked head has a single block and the main chain
// consists of this single block and another block together forming a tipset
// that is a superset of the forked head.
func getSubsetOldNewCommon(ctx context.Context, t *testing.T, chainStore *chain.Store, blockSource *th.TestFetcher, dstP *DefaultSyncerTestParams, a uint) (types.TipSet, types.TipSet, types.TipSet) {
	requireGrowChain(ctx, t, blockSource, chainStore, a, dstP)
	commonAncestor := requireHeadTipset(t, chainStore)
	requireGrowChain(ctx, t, blockSource, chainStore, 1, dstP)
	oldHead := requireHeadTipset(t, chainStore)
	headBlock := oldHead.ToSlice()[0]

	signer, ki := types.NewMockSignersAndKeyInfo(1)
	mockSignerPubKey := ki[0].PublicKey()
	block2 := th.RequireMkFakeChild(t, th.FakeChildParams{
		Parent:      commonAncestor,
		MinerPubKey: mockSignerPubKey,
		Signer:      signer,
		GenesisCid:  dstP.genCid,
		StateRoot:   dstP.genStateRoot})
	requirePutBlocks(t, blockSource, block2)
	superset := th.RequireNewTipSet(t, headBlock, block2)
	supersetTsas := &chain.TipSetAndState{
		TipSet:          superset,
		TipSetStateRoot: dstP.genStateRoot,
	}
	require.NoError(t, chainStore.PutTipSetAndState(ctx, supersetTsas))

	return oldHead, superset, commonAncestor
}
