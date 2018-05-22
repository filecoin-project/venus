package types

import (
	"errors"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"
)

func init() {
	cbor.RegisterCborType(MessageReceipt{})
}

// ErrReturnValueTooLarge is returned if a return value is larger than the maximum.
var ErrReturnValueTooLarge = errors.New("return value was too large, must be <= 64 bytes")

// ReturnValueLength is the length of a return value in a message receipt.
const ReturnValueLength = 64

// MessageReceipt represents the result of sending a message.
type MessageReceipt struct {
	// `0` is success, anything else is an error code in unix style.
	ExitCode uint8 `json:"exitCode"`

	// Return is the value returned or an error message.
	Return [ReturnValueLength]byte `json:"return"`

	// ReturnSize is the size of the actual return value.
	ReturnSize uint `json:"returnSize"`
}

// NewMessageReceipt creates a new MessageReceipt.
func NewMessageReceipt(exitCode uint8, ret [ReturnValueLength]byte, retSize uint) *MessageReceipt {
	return &MessageReceipt{
		ExitCode:   exitCode,
		Return:     ret,
		ReturnSize: retSize,
	}
}

// ReturnValue returns the return value, truncated according to the ReturnSize.
func (r *MessageReceipt) ReturnValue() []byte {
	return r.Return[0:r.ReturnSize]
}

// SliceToReturnValue takes a bytes slice and converts into its length and a fixed size
// byte array.
func SliceToReturnValue(ret []byte) ([ReturnValueLength]byte, uint, error) {
	var val [ReturnValueLength]byte

	if len(ret) > ReturnValueLength {
		return val, 0, ErrReturnValueTooLarge
	}

	copy(val[:], ret)
	return val, uint(len(ret)), nil
}
