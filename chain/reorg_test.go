package chain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestIsReorgFork(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)

	// main chain has 3 blocks past CA, fork has 1
	old, new, common := getForkOldNewCommon(ctx, t, builder, 2, 3, 1)
	assert.True(t, chain.IsReorg(old, new, common))
}
func TestIsReorgPrefix(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	// Old head is a direct ancestor of new head
	old, new, common := getForkOldNewCommon(ctx, t, builder, 2, 3, 0)
	assert.False(t, chain.IsReorg(old, new, common))
}

func TestIsReorgSubset(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	old, new, common := getSubsetOldNewCommon(ctx, t, builder, 2)
	assert.False(t, chain.IsReorg(old, new, common))
}

func TestReorgDiffFork(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	// main chain has 11 blocks past CA, fork has 10
	old, new, common := getForkOldNewCommon(ctx, t, builder, 10, 11, 10)

	dropped, added, err := chain.ReorgDiff(old, new, common)
	assert.NoError(t, err)
	assert.Equal(t, uint64(10), dropped)
	assert.Equal(t, uint64(11), added)
}

func TestReorgDiffSubset(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	old, new, common := getSubsetOldNewCommon(ctx, t, builder, 10)

	dropped, added, err := chain.ReorgDiff(old, new, common)
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), dropped)
	assert.Equal(t, uint64(1), added)
}

// getForkOldNewCommon is a testing helper function that creates chain with the builder.
// The blockchain forks and the common ancestor block is 'a' (> 0) blocks after the genesis block.
// The  main chain has an additional 'b' blocks, the fork has an additional 'c' blocks.
// This function returns the forked head, the main head and the common ancestor.
func getForkOldNewCommon(ctx context.Context, t *testing.T, builder *chain.Builder, a, b, c int) (types.TipSet, types.TipSet, types.TipSet) {
	// Add "a" tipsets to the head of the chainStore.
	commonHead := builder.AppendManyOn(a, types.UndefTipSet)
	oldHead := commonHead

	if c > 0 {
		oldHead = builder.AppendManyOn(c, commonHead)
	}
	newHead := builder.AppendManyOn(b, commonHead)
	return oldHead, newHead, commonHead
}

// getSubsetOldNewCommon is a testing helper function that creates and stores
// a blockchain in the chainStore.  The blockchain has 'a' blocks after genesis
// and then a fork.  The forked head has a single block and the main chain
// consists of this single block and another block together forming a tipset
// that is a superset of the forked head.
func getSubsetOldNewCommon(ctx context.Context, t *testing.T, builder *chain.Builder, a int) (types.TipSet, types.TipSet, types.TipSet) {
	commonHead := builder.AppendManyBlocksOnBlocks(a)
	block1 := builder.AppendBlockOnBlocks(commonHead)
	block2 := builder.AppendBlockOnBlocks(commonHead)

	oldHead := types.RequireNewTipSet(t, block1)
	superset := types.RequireNewTipSet(t, block1, block2)
	return oldHead, superset, types.RequireNewTipSet(t, commonHead)
}
