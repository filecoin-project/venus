package chain

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/venus/venus-shared/testutil"
)

func TestSignedMessageBasic(t *testing.T) {
	sliceLen := 16
	bytesLen := 32

	var buf bytes.Buffer
	for i := 0; i < 32; i++ {
		var src, dst SignedMessage

		opt := testutil.CborErBasicTestOptions{
			Buf: &buf,
			Prepare: func() {
				require.Equal(t, src, dst)
				require.Nil(t, src.Signature.Data)
			},

			ProvideOpts: []interface{}{
				testutil.WithSliceLen(sliceLen),
				testutil.BytesFixedProvider(bytesLen),
			},

			Provided: func() {
				require.NotEqual(t, src, dst, "value provided")
				require.Len(t, src.Signature.Data, bytesLen)
			},

			Finished: func() {
				require.Equal(t, src, dst, "after unmarshaling")
				require.Equal(t, src.String(), dst.String())

				c := src.Cid()

				blk, err := src.ToStorageBlock()
				require.NoError(t, err, "ToStorageBlock")

				require.Equal(t, c, blk.Cid())
				require.Equal(t, c, dst.Cid())

				switch src.Signature.Type {
				case crypto.SigTypeBLS:
					require.Equal(t, c, src.Message.Cid())
					require.Equal(t, src.ChainLength(), src.Message.ChainLength())

				case crypto.SigTypeSecp256k1:
					require.NotEqual(t, c, src.Message.Cid())
					require.Greater(t, src.ChainLength(), src.Message.ChainLength())

				default:
					t.Fatalf("unexpected sig type %d", src.Signature.Type)
				}
			},
		}

		testutil.CborErBasicTest(t, &src, &dst, opt)
	}
}
