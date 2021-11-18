package chain

import (
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/venus-shared/testutil"
)

func TestTipSetKey(t *testing.T) {
	var cids []cid.Cid
	cidNum := 10

	// value provided
	testutil.Provide(t, &cids, testutil.WithSliceLen(cidNum))
	require.Len(t, cids, cidNum)
	require.NotEqual(t, make([]cid.Cid, cidNum), cids)

	// construct
	tsk := NewTipSetKey(cids...)
	require.False(t, tsk.IsEmpty())

	require.NotEqual(t, tsk, EmptyTSK)

	// content
	require.Equal(t, tsk.Cids(), cids)
	tskStr := tsk.String()
	for i := range cids {
		require.Contains(t, tskStr, cids[i].String())
	}

	// marshal json
	data, err := tsk.MarshalJSON()
	require.NoError(t, err, "marshal json")

	var decoded TipSetKey
	err = decoded.UnmarshalJSON(data)
	require.NoError(t, err)

	require.Equal(t, tsk, decoded)
}
