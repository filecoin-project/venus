// stm: #unit
package chain

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/stretchr/testify/require"
)

func TestChainIndex(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	builder := NewBuilder(t, address.Undef)
	genTS := builder.Genesis()

	linksCount := 20
	links := make([]*types.TipSet, linksCount)
	links[0] = genTS

	for i := 1; i < linksCount; i++ {
		links[i] = builder.AppendOn(ctx, links[i-1], rand.Intn(2)+1)
	}

	head := links[linksCount-1]

	DefaultChainIndexCacheSize = 10
	chainIndex := NewChainIndex(builder.GetTipSet)
	chainIndex.skipLength = 10

	// stm: @CHAIN_INDEX_GET_TIPSET_BY_HEIGHT_001, @CHAIN_INDEX_GET_TIPSET_BY_HEIGHT_002, @CHAIN_INDEX_GET_TIPSET_BY_HEIGHT_004
	for i := 0; i < linksCount; i++ {
		_, err := chainIndex.GetTipSetByHeight(ctx, head, abi.ChainEpoch(i))
		require.NoError(t, err)
	}

	// stm: @CHAIN_INDEX_GET_TIPSET_BY_HEIGHT_NO_CACHE_001
	_, err := chainIndex.GetTipsetByHeightWithoutCache(ctx, head, head.Height()/2)
	require.NoError(t, err)

	chainIndex.loadTipSet = func(_ context.Context, _ types.TipSetKey) (*types.TipSet, error) {
		return nil, fmt.Errorf("error round down")
	}

	// If error occurs after calling roundDown function
	// stm: @CHAIN_INDEX_GET_TIPSET_BY_HEIGHT_003
	_, err = chainIndex.GetTipSetByHeight(ctx, head, head.Height()/2)
	require.Error(t, err)
}
