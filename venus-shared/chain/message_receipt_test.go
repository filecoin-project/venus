package chain

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

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
				require.Equal(t, src, dst, "empty values")
				require.Nil(t, src.ReturnValue)
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
				require.Len(t, src.ReturnValue, dataLen)

				require.GreaterOrEqual(t, src.ExitCode, exitcode.ExitCode(0))
				require.Less(t, src.ExitCode, exitcode.ExitCode(20))

				require.GreaterOrEqual(t, src.GasUsed, int64(10_000_000))
				require.Less(t, src.GasUsed, int64(50_000_000))
			},

			Finished: func() {
				require.Equal(t, src, dst, "from src to dst through cbor")
				require.Equal(t, src.String(), dst.String(), "string representation")
			},
		}

		testutil.CborErBasicTest(t, &src, &dst, opt)
	}
}
