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
		opt := cborBasicTestOptions{
			buf: &buf,
			before: func() {
				assert.Equal(t, src, dst, "empty values")
			},

			provideOpts: []interface{}{
				testutil.BytesFixedProvider(paramsLen),
				testutil.BlsAddressProvider(),
			},

			provided: func() {
				assert.NotEqual(t, src, dst, "value provided")
				assert.Equal(t, src.From.Protocol(), address.BLS, "from addr proto")
				assert.Equal(t, src.To.Protocol(), address.BLS, "to addr proto")
				assert.Len(t, src.Params, paramsLen, "params length")
			},

			after: func() {
				assert.Equal(t, src, dst)
				assert.Equal(t, src.Cid(), dst.Cid())
			},
		}

		cborBasicTest(t, &src, &dst, opt)
	}
}
