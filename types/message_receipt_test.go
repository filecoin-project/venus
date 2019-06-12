package types

import (
	"testing"

	cbor "github.com/ipfs/go-ipld-cbor"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
)

func TestMessageReceiptMarshal(t *testing.T) {
	tf.UnitTest(t)

	cases := []MessageReceipt{
		{
			ExitCode: 1,
		},
		{
			ExitCode: 0,
			Return:   [][]byte{{1, 2, 3}},
		},
		{},
	}

	for _, expected := range cases {
		bytes, err := cbor.DumpObject(expected)
		assert.NoError(t, err)

		var actual MessageReceipt
		err = cbor.DecodeInto(bytes, &actual)

		assert.NoError(t, err)
		assert.Equal(t, expected.ExitCode, actual.ExitCode)
		assert.Equal(t, expected.Return, actual.Return)
		assert.True(t, expected.GasAttoFIL.Equal(actual.GasAttoFIL))
	}
}
