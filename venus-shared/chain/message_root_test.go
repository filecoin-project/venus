package chain

import (
	"bytes"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/venus/venus-shared/testutil"
)

func TestMessageRootBasic(t *testing.T) {
	var buf bytes.Buffer
	for i := 0; i < 32; i++ {
		var src, dst MessageRoot

		opt := testutil.CborErBasicTestOptions{
			Buf: &buf,
			Prepare: func() {
				assert.Equal(t, src, dst, "empty values")
				assert.Equal(t, src.BlsRoot, cid.Undef)
				assert.Equal(t, src.SecpkRoot, cid.Undef)
			},

			Provided: func() {
				assert.NotEqual(t, src.BlsRoot, cid.Undef)
				assert.NotEqual(t, src.SecpkRoot, cid.Undef)
			},

			Finished: func() {
				assert.Equal(t, src, dst, "from src to dst through cbor")

				blk, err := src.ToStorageBlock()
				assert.NoError(t, err, "ToStorageBlock")

				srcCid := src.Cid()
				assert.Equal(t, srcCid, dst.Cid(), "cid compare to dst")
				assert.Equal(t, srcCid, blk.Cid(), "cid compare to sblk")
			},
		}

		testutil.CborErBasicTest(t, &src, &dst, opt)
	}
}
