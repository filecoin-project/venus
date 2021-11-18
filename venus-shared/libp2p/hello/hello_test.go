package hello

import (
	"bytes"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/venus-shared/testutil"
)

func TestGreetingMessage(t *testing.T) {
	var buf bytes.Buffer
	sliceLen := 5

	for i := 0; i < 32; i++ {
		var src, dst GreetingMessage

		opt := testutil.CborErBasicTestOptions{
			Buf: &buf,
			Prepare: func() {
				require.Equal(t, src, dst, "empty values")
			},

			ProvideOpts: []interface{}{
				testutil.WithSliceLen(sliceLen),
				testutil.PositiveBigProvider(),
			},

			Provided: func() {
				require.Len(t, src.HeaviestTipSet, sliceLen, "HeaviestTipSet length")
				require.True(t, src.HeaviestTipSetWeight.GreaterThan(big.Zero()), "positive HeaviestTipSetWeight")
				require.NotEqual(t, src.GenesisHash, cid.Undef, "GenesisHash")
			},

			Finished: func() {
				require.Equal(t, src, dst, "from src to dst through cbor")
			},
		}

		testutil.CborErBasicTest(t, &src, &dst, opt)
	}
}

func TestLatencyMessage(t *testing.T) {
	var buf bytes.Buffer

	for i := 0; i < 32; i++ {
		var src, dst LatencyMessage

		opt := testutil.CborErBasicTestOptions{
			Buf: &buf,
			Prepare: func() {
				require.Equal(t, src, dst, "empty values")
			},

			ProvideOpts: []interface{}{
				testutil.IntRangedProvider(100, 200),
			},

			Provided: func() {
				require.GreaterOrEqual(t, src.TArrival, int64(100), "LatencyMessage.TArrival min")
				require.Less(t, src.TArrival, int64(200), "LatencyMessage.TArrival max")

				require.GreaterOrEqual(t, src.TSent, int64(100), "LatencyMessage.TSent min")
				require.Less(t, src.TSent, int64(200), "LatencyMessage.TSent max")
			},

			Finished: func() {
				require.Equal(t, src, dst, "from src to dst through cbor")
			},
		}

		testutil.CborErBasicTest(t, &src, &dst, opt)
	}
}
