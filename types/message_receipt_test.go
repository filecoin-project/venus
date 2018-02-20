package types

import (
	"testing"

	cbor "gx/ipfs/QmZpue627xQuNGXn7xHieSjSZ8N4jot6oBHwe9XTn3e4NU/go-ipld-cbor"

	"github.com/stretchr/testify/assert"
)

func TestMessageReceiptMarshal(t *testing.T) {
	assert := assert.New(t)

	c1, err := cidFromString("hello")
	assert.NoError(err)

	receipt := NewMessageReceipt(c1, 8, []byte{1, 2, 3})
	bytes, err := cbor.DumpObject(receipt)
	assert.NoError(err)

	var receiptBack MessageReceipt
	err = cbor.DecodeInto(bytes, &receiptBack)
	assert.NoError(err)

	assert.Equal(receipt, &receiptBack)
}
