package core_test

// FIXME: https://github.com/filecoin-project/go-filecoin/issues/1541
/*import (
	"context"
	"testing"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func countBlocks(chm *chain.Store) (count int) {
	for range chm.BlockHistory(context.Background()) {
		count++
	}
	return count
}

func nullBlockExists(require *require.Assertions, cm *ChainManager) bool {
	historyCh := cm.BlockHistory(context.Background())
	raw := <-historyCh
	ts, ok := raw.(TipSet)
	require.True(ok)
	height, err := ts.Height()
	require.NoError(err)
	prevHeight := uint64(height)
	for raw := range historyCh {
		ts, ok := raw.(TipSet)
		require.True(ok)
		height, err = ts.Height()
		require.NoError(err)
		if prevHeight-uint64(height) > 1 {
			return true
		}
		prevHeight = uint64(height)
	}
	return false
}

func heightsAreOrdered(require *require.Assertions, cm *ChainManager) bool {
	historyCh := cm.BlockHistory(context.Background())
	raw := <-historyCh
	ts, ok := raw.(TipSet)
	require.True(ok)
	height, err := ts.Height()
	require.NoError(err)
	prevHeight := uint64(height)
	for raw := range historyCh {
		ts, ok := raw.(TipSet)
		require.True(ok)
		height, err := ts.Height()
		require.NoError(err)

		if prevHeight < uint64(height) {
			return false
		}
		prevHeight = uint64(height)
	}
	return true
}

func multiBlockTipSetExists(require *require.Assertions, cm *ChainManager) bool {
	for raw := range cm.BlockHistory(context.Background()) {
		ts, ok := raw.(TipSet)
		require.True(ok)
		if len(ts) > 1 {
			return true
		}
	}
	return false
}

func TestAddChain(t *testing.T) {
	assert := assert.New(t)
	ctx, _, _, cm := newTestUtils()
	assert.NoError(cm.Genesis(ctx, InitGenesis))

	assert.Equal(1, countBlocks(cm))

	bts := cm.GetHeaviestTipSet()
	stateGetter := func(ctx context.Context, ts TipSet) (state.Tree, error) {
		return cm.State(ctx, ts.ToSlice())
	}
	_, err := AddChain(ctx, cm.ProcessNewBlock, stateGetter, bts.ToSlice(), 9)
	assert.NoError(err)

	assert.Equal(10, countBlocks(cm))
}

func TestAddChainBinomBlocksPerEpoch(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx, _, _, cm := newTestUtils()
	assert.NoError(cm.Genesis(ctx, InitGenesis))

	assert.Equal(1, countBlocks(cm))
	hts := cm.GetHeaviestTipSet()
	startHeight, err := hts.Height()
	require.NoError(err)

	stateGetter := func(ctx context.Context, ts TipSet) (state.Tree, error) {
		return cm.State(ctx, ts.ToSlice())
	}
	_, err = AddChainBinomBlocksPerEpoch(ctx, cm.ProcessNewBlock, stateGetter, hts, 100, 199)
	assert.NoError(err)
	hts = cm.GetHeaviestTipSet()
	endHeight, err := hts.Height()
	require.NoError(err)

	assert.True(countBlocks(cm) <= 200)

	// With overwhelming proability there will be null blocks and tipsets
	// with multiple blocks
	assert.True(multiBlockTipSetExists(require, cm))
	assert.True(nullBlockExists(require, cm))

	// With high probability the last 10 blocks will not be null blocks
	// so the final height should be within 10 of 200
	assert.True(uint64(endHeight)-uint64(startHeight) > uint64(190))

	assert.True(heightsAreOrdered(require, cm))
}
*/
