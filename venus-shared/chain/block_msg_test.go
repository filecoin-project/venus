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

		opt := testutil.CborErBasicTestOptions{
			Buf: &buf,
			Prepare: func() {
				assert.Equal(t, src, dst)
				assert.Nil(t, src.Header)
				assert.Nil(t, src.BlsMessages)
				assert.Nil(t, src.SecpkMessages)
			},

			ProvideOpts: []interface{}{
				testutil.WithSliceLen(msgLen),
			},

			Provided: func() {
				assert.NotEqual(t, src, dst, "value provided")
				assert.NotNil(t, src.Header)
				assert.NotEqual(t, emptyCids, src.BlsMessages)
				assert.NotEqual(t, emptyCids, src.SecpkMessages)
			},

			Marshaled: func(b []byte) {
				bmCid := src.Cid()
				assert.Equal(t, bmCid, src.Header.Cid(), "Cid() result for BlockMsg")

				sumCid, err := abi.CidBuilder.Sum(b)
				assert.NoError(t, err, "CidBuilder.Sum")

				assert.NotEqual(t, bmCid, sumCid)

				serialized, err := src.Serialize()
				assert.NoError(t, err, "Serialize")
				assert.Equal(t, b, serialized)
			},

			Finished: func() {
				assert.Equal(t, src, dst, "after unmarshaling")
			},
		}

		testutil.CborErBasicTest(t, &src, &dst, opt)
	}
}
