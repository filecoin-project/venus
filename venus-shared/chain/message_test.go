package chain

import (
	"bytes"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	blocks "github.com/ipfs/go-block-format"
	"github.com/stretchr/testify/assert"
)

func TestMessageBasic(t *testing.T) {
	paramsLen := 32
	var buf bytes.Buffer
	for i := 0; i < 32; i++ {
		var src, dst Message
		var blk blocks.Block
		opt := testutil.CborErBasicTestOptions{
			Buf: &buf,
			Prepare: func() {
				assert.Equal(t, src, dst, "empty values")
			},

			ProvideOpts: []interface{}{
				testutil.BytesFixedProvider(paramsLen),
				testutil.BlsAddressProvider(),
			},

			Provided: func() {
				assert.NotEqual(t, src, dst, "value provided")
				assert.Equal(t, src.From.Protocol(), address.BLS, "from addr proto")
				assert.Equal(t, src.To.Protocol(), address.BLS, "to addr proto")
				assert.Len(t, src.Params, paramsLen, "params length")

				src.Version = MessageVersion

				sblk, err := src.ToStorageBlock()
				assert.NoError(t, err, "ToStorageBlock")
				blk = sblk
			},

			Marshaled: func(b []byte) {
				decoded, err := DecodeMessage(b)
				assert.NoError(t, err, "DecodeMessage")
				assert.True(t, src.Equals(decoded))
			},

			Finished: func() {
				assert.Equal(t, src, dst)
				assert.True(t, src.Equals(&dst))
				assert.True(t, src.EqualCall(&dst))
				assert.Equal(t, src.Cid(), dst.Cid())
				assert.Equal(t, src.Cid(), blk.Cid())
				assert.Equal(t, src.String(), dst.String())
			},
		}

		testutil.CborErBasicTest(t, &src, &dst, opt)
	}
}
