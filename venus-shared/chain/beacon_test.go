package chain

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/venus/venus-shared/testutil"
)

func TestBeaconEntryBasic(t *testing.T) {
	dataLen := 32

	var buf bytes.Buffer
	for i := 0; i < 32; i++ {
		var src, dst BeaconEntry

		opt := testutil.CborErBasicTestOptions{
			Buf: &buf,
			Prepare: func() {
				assert.Equal(t, src, dst, "empty values")
				assert.Nil(t, src.Data)
			},

			ProvideOpts: []interface{}{
				testutil.BytesFixedProvider(dataLen),
			},

			Provided: func() {
				assert.Len(t, src.Data, dataLen)
			},

			Finished: func() {
				assert.Equal(t, src, dst, "from src to dst through cbor")

			},
		}

		testutil.CborErBasicTest(t, &src, &dst, opt)
	}
}
