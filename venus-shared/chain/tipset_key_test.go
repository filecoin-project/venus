package chain

import (
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/venus/venus-shared/testutil"
)

func TestTipSetKey(t *testing.T) {
	var cids []cid.Cid
	cidNum := 10

	// value provided
	testutil.Provide(t, &cids, testutil.WithSliceLen(cidNum))
	assert.Len(t, cids, cidNum)
	assert.NotEqual(t, make([]cid.Cid, cidNum), cids)

	// construct
	tsk := NewTipSetKey(cids...)
	assert.False(t, tsk.IsEmpty())

	assert.NotEqual(t, tsk, EmptyTSK)

	// content
	assert.Equal(t, tsk.Cids(), cids)
	tskStr := tsk.String()
	for i := range cids {
		assert.Contains(t, tskStr, cids[i].String())
	}

	// marshal json
	data, err := tsk.MarshalJSON()
	assert.NoError(t, err, "marshal json")

	var decoded TipSetKey
	err = decoded.UnmarshalJSON(data)
	assert.NoError(t, err)

	assert.Equal(t, tsk, decoded)
}
