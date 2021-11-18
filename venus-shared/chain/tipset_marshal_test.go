package chain

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/filecoin-project/venus/venus-shared/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func checkForTipSetEqual(t *testing.T, a, b *TipSet) {
	require.True(t, a.Equals(b))
	require.Equal(t, a.Len(), b.Len(), "Len equals")
	require.Equal(t, a.Cids(), b.Cids(), "Cids equals")
	require.Equal(t, a.MinTimestamp(), b.MinTimestamp(), "MinTimestamp equals")
	require.Equal(t, a.ParentState(), b.ParentState(), "ParentState equals")
	require.Equal(t, a.String(), b.String(), "String equals")
	require.Equal(t, a.MinTicketBlock(), b.MinTicketBlock(), "MinTicketBlock equals")
	require.Equal(t, a.MinTicket(), b.MinTicket(), "MinTicket equals")
}

func TestTipSetMarshalJSON(t *testing.T) {
	height, paretns, weight := constructTipSetKeyInfos(t)
	ts := constructTipSet(t, height, paretns, weight)

	data, err := json.Marshal(ts)
	require.NoError(t, err, "json mahrshal for TipSet")

	var dst TipSet
	err = json.Unmarshal(data, &dst)
	require.NoError(t, err, "json unmarshal for TipSet")

	checkForTipSetEqual(t, ts, &dst)
}

func TestTipSetEquals(t *testing.T) {
	height, paretns, weight := constructTipSetKeyInfos(t)
	ts := constructTipSet(t, height, paretns, weight)

	assert.True(t, (*TipSet)(nil).Equals(nil), "nil tipset equals")
	assert.False(t, ts.Equals(nil), "non-nil is always != nil")
}

func TestTipSetBasic(t *testing.T) {
	var buf bytes.Buffer

	for i := 0; i < 32; i++ {
		height, paretns, weight := constructTipSetKeyInfos(t)
		ts := constructTipSet(t, height, paretns, weight)

		var src, dst TipSet

		opts := testutil.CborErBasicTestOptions{
			Buf: &buf,

			Provided: func() {
				// all fields in TipSet are private, so we assign the value manually
				src = *ts
				require.NotEqual(t, src, dst, "src provided")
			},

			Finished: func() {
				require.Equal(t, src, dst, "struct equals")
				checkForTipSetEqual(t, &src, &dst)

				src.height++
				require.False(t, src.Equals(&dst), "height matters in TipSet.Equals")
			},
		}

		testutil.CborErBasicTest(t, &src, &dst, opts)
	}
}
