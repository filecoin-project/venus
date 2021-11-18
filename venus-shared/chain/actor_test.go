package chain

import (
	"bytes"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/venus/venus-shared/testutil"
)

func TestActorBasic(t *testing.T) {
	var buf bytes.Buffer
	for i := 0; i < 32; i++ {
		var src, dst Actor

		opt := testutil.CborErBasicTestOptions{
			Buf: &buf,
			Prepare: func() {
				assert.Equal(t, src, dst, "empty values")
			},

			Provided: func() {
				assert.NotEqual(t, src.Code, cid.Undef)
				assert.NotEqual(t, src.Head, cid.Undef)
			},

			Finished: func() {
				assert.Equal(t, src, dst, "from src to dst through cbor")

			},
		}

		testutil.CborErBasicTest(t, &src, &dst, opt)
	}
}
