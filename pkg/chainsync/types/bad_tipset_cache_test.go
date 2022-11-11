// stm: #unit
package types

import (
	"testing"

	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/stretchr/testify/assert"
)

func TestBadTipsetCache(t *testing.T) {
	tf.UnitTest(t)
	badTSCache := NewBadTipSetCache()

	var ts types.TipSet
	testutil.Provide(t, &ts)

	// stm: @CHAINSYNC_TYPES_ADD_CHAIN_001
	badTSCache.AddChain([]*types.TipSet{&ts})

	var tsKey types.TipSetKey
	testutil.Provide(t, &tsKey, testutil.WithSliceLen(3))

	// stm: @CHAINSYNC_TYPES_ADD_001
	badTSCache.Add(tsKey.String())

	// stm: @CHAINSYNC_TYPES_HAS_001
	assert.True(t, badTSCache.Has(ts.Key().String()))
	assert.True(t, badTSCache.Has(tsKey.String()))
}
