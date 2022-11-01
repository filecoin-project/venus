// stm: #unit
package types

import (
	"testing"

	"github.com/filecoin-project/venus/venus-shared/testutil"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/stretchr/testify/assert"
)

func TestBadTipsetCache(t *testing.T) {
	badTsCache := NewBadTipSetCache()

	var ts types.TipSet
	testutil.Provide(t, &ts)

	// stm: @CHAINSYNC_TYPES_ADD_CHAIN_001
	badTsCache.AddChain([]*types.TipSet{&ts})

	var tsKey types.TipSetKey
	testutil.Provide(t, &tsKey, testutil.WithSliceLen(3))

	// stm: @CHAINSYNC_TYPES_ADD_001
	badTsCache.Add(tsKey.String())

	// stm: @CHAINSYNC_TYPES_HAS_001
	assert.True(t, badTsCache.Has(ts.Key().String()))
	assert.True(t, badTsCache.Has(tsKey.String()))
}
