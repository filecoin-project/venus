package chain

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

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
				assert.Equal(t, src, dst, "empty values")
				assert.Nil(t, src.Cids, "empty cids")
				assert.Nil(t, src.Blocks, "empty blocks")
			},

			ProvideOpts: []interface{}{
				testutil.WithSliceLen(sliceLen),
				testutil.BytesFixedProvider(bytesLen),
			},

			Provided: func() {
				assert.NotEqual(t, src, dst, "src value provided")
				assert.Len(t, src.Cids, sliceLen, "cids length")
				assert.Len(t, src.Blocks, sliceLen, "blocks length")
			},

			Finished: func() {
				assert.Equal(t, src, dst)
			},
		}

		testutil.CborErBasicTest(t, &src, &dst, opt)
	}
}
