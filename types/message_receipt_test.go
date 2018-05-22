package types

import (
	"testing"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"

	"github.com/stretchr/testify/assert"
)

func TestMessageReceiptMarshal(t *testing.T) {
	assert := assert.New(t)

	retVal, retSize, err := SliceToReturnValue([]byte{1, 2, 3})
	assert.NoError(err)

	receipt := NewMessageReceipt(8, retVal, retSize)
	bytes, err := cbor.DumpObject(receipt)
	assert.NoError(err)

	var receiptBack MessageReceipt
	err = cbor.DecodeInto(bytes, &receiptBack)
	assert.NoError(err)

	assert.Equal(receipt, &receiptBack)
}

func TestSliceToReturnValue(t *testing.T) {
	assert := assert.New(t)

	// empty
	ret, size, err := SliceToReturnValue([]byte{})
	assert.NoError(err)
	assert.Equal(uint(0), size)
	assert.Equal(make([]byte, 64), ret[:])

	ret, size, err = SliceToReturnValue(nil)
	assert.NoError(err)
	assert.Equal(uint(0), size)
	assert.Equal(make([]byte, 64), ret[:])

	// smaller than 64 bytes
	ret, size, err = SliceToReturnValue([]byte{1, 2, 3})
	assert.NoError(err)
	assert.Equal(uint(3), size)
	assert.Equal([]byte{1, 2, 3}, ret[0:3])
	var empty [61]byte
	assert.Equal(empty[:], ret[3:])

	// exactly 64 bytes
	input := make([]byte, 64)
	for i := range input {
		input[i] = byte(i)
	}
	ret, size, err = SliceToReturnValue(input)
	assert.NoError(err)
	assert.Equal(uint(64), size)
	assert.Equal(input, ret[:])

	// larger than 64 bytes
	input = make([]byte, 128)
	for i := range input {
		input[i] = byte(i)
	}
	_, _, err = SliceToReturnValue(input)
	assert.EqualError(err, ErrReturnValueTooLarge.Error())
}
