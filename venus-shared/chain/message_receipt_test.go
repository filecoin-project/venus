package chain

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/venus/venus-shared/testutil"
)

func TestMessageReceiptBasic(t *testing.T) {
	dataLen := 32

	var buf bytes.Buffer
	for i := 0; i < 32; i++ {
		var src, dst MessageReceipt

		opt := testutil.CborErBasicTestOptions{
			Buf: &buf,
			Prepare: func() {
				assert.Equal(t, src, dst, "empty values")
				assert.Nil(t, src.ReturnValue)
			},

			ProvideOpts: []interface{}{
				testutil.BytesFixedProvider(dataLen),
				testutil.IntRangedProvider(10_000_000, 50_000_000),
				func(t *testing.T) exitcode.ExitCode {
					p := testutil.IntRangedProvider(0, 20)
					next := p(t)
					return exitcode.ExitCode(next)
				},
			},

			Provided: func() {
				assert.Len(t, src.ReturnValue, dataLen)

				assert.GreaterOrEqual(t, src.ExitCode, exitcode.ExitCode(0))
				assert.Less(t, src.ExitCode, exitcode.ExitCode(20))

				assert.GreaterOrEqual(t, src.GasUsed, int64(10_000_000))
				assert.Less(t, src.GasUsed, int64(50_000_000))
			},

			Finished: func() {
				assert.Equal(t, src, dst, "from src to dst through cbor")
				assert.Equal(t, src.String(), dst.String(), "string representation")
			},
		}

		testutil.CborErBasicTest(t, &src, &dst, opt)
	}
}
