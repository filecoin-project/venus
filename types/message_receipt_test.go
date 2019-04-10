package types

import (
	"testing"

	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/assert"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

func TestMessageReceiptMarshal(t *testing.T) {
	tf.UnitTest(t)

	assert := assert.New(t)

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
		assert.NoError(err)

		var actual MessageReceipt
		err = cbor.DecodeInto(bytes, &actual)

		assert.NoError(err)
		assert.Equal(expected, actual)
	}
}
