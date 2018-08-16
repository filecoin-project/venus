package types

import (
	"testing"

	cbor "gx/ipfs/QmPbqRavwDZLfmpeW6eoyAoQ5rT2LoCW98JhvRc22CqkZS/go-ipld-cbor"

	"github.com/stretchr/testify/assert"
)

func TestMessageReceiptMarshal(t *testing.T) {
	assert := assert.New(t)

	cases := []MessageReceipt{
		{
			ExitCode: 1,
		},
		{
			ExitCode: 0,
			Return:   []Bytes{[]byte{1, 2, 3}},
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
