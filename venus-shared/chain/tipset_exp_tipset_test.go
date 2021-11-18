package chain

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/venus-shared/testutil"
)

func TestExpTipSet(t *testing.T) {
	sliceLen := 5
	bytesLen := 32

	var buf bytes.Buffer
	for i := 0; i < 32; i++ {
		var src, dst ExpTipSet

		opt := testutil.CborErBasicTestOptions{
			Buf: &buf,
			Prepare: func() {
				require.Equal(t, src, dst, "empty values")
				require.Nil(t, src.Cids, "empty cids")
				require.Nil(t, src.Blocks, "empty blocks")
			},

			ProvideOpts: []interface{}{
				testutil.WithSliceLen(sliceLen),
				testutil.BytesFixedProvider(bytesLen),
			},

			Provided: func() {
				require.NotEqual(t, src, dst, "src value provided")
				require.Len(t, src.Cids, sliceLen, "cids length")
				require.Len(t, src.Blocks, sliceLen, "blocks length")
			},

			Finished: func() {
				require.Equal(t, src, dst)
			},
		}

		testutil.CborErBasicTest(t, &src, &dst, opt)
	}
}
