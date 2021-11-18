package chain

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

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
				assert.Equal(t, src, dst)
				assert.Nil(t, src.Signature.Data)
			},

			ProvideOpts: []interface{}{
				testutil.WithSliceLen(sliceLen),
				testutil.BytesFixedProvider(bytesLen),
			},

			Provided: func() {
				assert.NotEqual(t, src, dst, "value provided")
				assert.Len(t, src.Signature.Data, bytesLen)
			},

			Finished: func() {
				assert.Equal(t, src, dst, "after unmarshaling")
				assert.Equal(t, src.String(), dst.String())

				c := src.Cid()

				blk, err := src.ToStorageBlock()
				assert.NoError(t, err, "ToStorageBlock")

				assert.Equal(t, c, blk.Cid())
				assert.Equal(t, c, dst.Cid())

				switch src.Signature.Type {
				case crypto.SigTypeBLS:
					assert.Equal(t, c, src.Message.Cid())
					assert.Equal(t, src.ChainLength(), src.Message.ChainLength())

				case crypto.SigTypeSecp256k1:
					assert.NotEqual(t, c, src.Message.Cid())
					assert.Greater(t, src.ChainLength(), src.Message.ChainLength())

				default:
					t.Fatalf("unexpected sig type %d", src.Signature.Type)
				}
			},
		}

		testutil.CborErBasicTest(t, &src, &dst, opt)
	}
}
