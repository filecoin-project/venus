package chain

import (
	"bytes"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	blocks "github.com/ipfs/go-block-format"
	"github.com/stretchr/testify/require"
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
				require.Equal(t, src, dst, "empty values")
			},

			ProvideOpts: []interface{}{
				testutil.BytesFixedProvider(paramsLen),
				testutil.BlsAddressProvider(),
			},

			Provided: func() {
				require.NotEqual(t, src, dst, "value provided")
				require.Equal(t, src.From.Protocol(), address.BLS, "from addr proto")
				require.Equal(t, src.To.Protocol(), address.BLS, "to addr proto")
				require.Len(t, src.Params, paramsLen, "params length")

				src.Version = MessageVersion

				sblk, err := src.ToStorageBlock()
				require.NoError(t, err, "ToStorageBlock")
				blk = sblk
			},

			Marshaled: func(b []byte) {
				decoded, err := DecodeMessage(b)
				require.NoError(t, err, "DecodeMessage")
				require.True(t, src.Equals(decoded))
			},

			Finished: func() {
				require.Equal(t, src, dst)
				require.True(t, src.Equals(&dst))
				require.True(t, src.EqualCall(&dst))
				require.Equal(t, src.Cid(), dst.Cid())
				require.Equal(t, src.Cid(), blk.Cid())
				require.Equal(t, src.String(), dst.String())
			},
		}

		testutil.CborErBasicTest(t, &src, &dst, opt)
	}
}
