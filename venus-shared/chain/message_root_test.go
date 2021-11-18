package chain

import (
	"bytes"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/venus-shared/testutil"
)

func TestMessageRootBasic(t *testing.T) {
	var buf bytes.Buffer
	for i := 0; i < 32; i++ {
		var src, dst MessageRoot

		opt := testutil.CborErBasicTestOptions{
			Buf: &buf,
			Prepare: func() {
				require.Equal(t, src, dst, "empty values")
				require.Equal(t, src.BlsRoot, cid.Undef)
				require.Equal(t, src.SecpkRoot, cid.Undef)
			},

			Provided: func() {
				require.NotEqual(t, src.BlsRoot, cid.Undef)
				require.NotEqual(t, src.SecpkRoot, cid.Undef)
			},

			Finished: func() {
				require.Equal(t, src, dst, "from src to dst through cbor")

				blk, err := src.ToStorageBlock()
				require.NoError(t, err, "ToStorageBlock")

				srcCid := src.Cid()
				require.Equal(t, srcCid, dst.Cid(), "cid compare to dst")
				require.Equal(t, srcCid, blk.Cid(), "cid compare to sblk")
			},
		}

		testutil.CborErBasicTest(t, &src, &dst, opt)
	}
}
