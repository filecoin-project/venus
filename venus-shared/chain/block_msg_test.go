package chain

import (
	"bytes"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/venus-shared/testutil"
)

func TestBlockMsgBasic(t *testing.T) {
	msgLen := 16
	emptyCids := make([]cid.Cid, msgLen)

	var buf bytes.Buffer
	for i := 0; i < 32; i++ {
		var src, dst BlockMsg

		opt := cborBasicTestOptions{
			buf: &buf,
			before: func() {
				assert.Equal(t, src, dst)
				assert.Nil(t, src.Header)
				assert.Nil(t, src.BlsMessages)
				assert.Nil(t, src.SecpkMessages)
			},

			provideOpts: []interface{}{
				testutil.WithSliceLen(msgLen),
			},

			provided: func() {
				assert.NotEqual(t, src, dst, "value provided")
				assert.NotNil(t, src.Header)
				assert.NotEqual(t, emptyCids, src.BlsMessages)
				assert.NotEqual(t, emptyCids, src.SecpkMessages)
			},

			marshaled: func(b []byte) {
				bmCid := src.Cid()
				assert.Equal(t, bmCid, src.Header.Cid(), "Cid() result for BlockMsg")

				sumCid, err := abi.CidBuilder.Sum(b)
				assert.NoError(t, err, "CidBuilder.Sum")

				assert.NotEqual(t, bmCid, sumCid)
			},

			after: func() {
				assert.Equal(t, src, dst, "after unmarshaling")
			},
		}

		cborBasicTest(t, &src, &dst, opt)
	}
}
