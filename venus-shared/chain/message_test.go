package chain

import (
	"bytes"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	"github.com/stretchr/testify/assert"
)

func TestMessageBasic(t *testing.T) {
	paramsLen := 32
	var buf bytes.Buffer
	for i := 0; i < 32; i++ {
		var src, dst Message
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
			},

			Finished: func() {
				assert.Equal(t, src, dst)
				assert.Equal(t, src.Cid(), dst.Cid())
			},
		}

		testutil.CborErBasicTest(t, &src, &dst, opt)
	}
}
