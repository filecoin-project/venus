package types

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-state-types/exitcode"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/venus-shared/testutil"
)

func TestMessageReceiptBasic(t *testing.T) {
	tf.UnitTest(t)
	dataLen := 32

	var buf bytes.Buffer
	for i := 0; i < 32; i++ {
		var src, dst MessageReceipt

		opt := testutil.CborErBasicTestOptions{
			Buf: &buf,
			Prepare: func() {
				require.Equal(t, src, dst, "empty values")
				require.Nil(t, src.Return)
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
				require.Len(t, src.Return, dataLen)

				require.GreaterOrEqual(t, src.ExitCode, exitcode.ExitCode(0))
				require.Less(t, src.ExitCode, exitcode.ExitCode(20))

				require.GreaterOrEqual(t, src.GasUsed, int64(10_000_000))
				require.Less(t, src.GasUsed, int64(50_000_000))
			},

			Finished: func() {
				// V0 version MessageReceipt does not contain EventsRoot field
				src.EventsRoot = nil
				require.Equal(t, src, dst, "from src to dst through cbor")
				require.Equal(t, src.String(), dst.String(), "string representation")
			},
		}

		testutil.CborErBasicTest(t, &src, &dst, opt)
	}
}

func TestMessageReceiptSerdeRoundrip(t *testing.T) {
	tf.UnitTest(t)
	var (
		assert = require.New(t)
		buf    = new(bytes.Buffer)
		err    error
	)

	randomCid, err := cid.Decode("bafy2bzacecu7n7wbtogznrtuuvf73dsz7wasgyneqasksdblxupnyovmtwxxu")
	assert.NoError(err)

	//
	// Version 0
	//
	mr := NewMessageReceiptV0(0, []byte{0x00, 0x01, 0x02, 0x04}, 42)

	// marshal
	err = mr.MarshalCBOR(buf)
	assert.NoError(err)

	t.Logf("version 0: %s\n", hex.EncodeToString(buf.Bytes()))

	// unmarshal
	var mr2 MessageReceipt
	err = mr2.UnmarshalCBOR(buf)
	assert.NoError(err)
	assert.Equal(mr, mr2)

	// version 0 with an events root -- should not serialize the events root!
	mr.EventsRoot = &randomCid

	buf.Reset()

	// marshal
	err = mr.MarshalCBOR(buf)
	assert.NoError(err)

	t.Logf("version 0 (with root): %s\n", hex.EncodeToString(buf.Bytes()))

	// unmarshal
	mr2 = MessageReceipt{}
	err = mr2.UnmarshalCBOR(buf)
	assert.NoError(err)
	assert.NotEqual(mr, mr2)
	assert.Nil(mr2.EventsRoot)

	//
	// Version 1
	//
	buf.Reset()
	mr = NewMessageReceiptV1(0, []byte{0x00, 0x01, 0x02, 0x04}, 42, &randomCid)

	// marshal
	err = mr.MarshalCBOR(buf)
	assert.NoError(err)

	t.Logf("version 1: %s\n", hex.EncodeToString(buf.Bytes()))

	// unmarshal
	mr2 = MessageReceipt{}
	err = mr2.UnmarshalCBOR(buf)
	assert.NoError(err)
	assert.Equal(mr, mr2)
	assert.NotNil(mr2.EventsRoot)
}
