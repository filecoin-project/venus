package cborutil

import (
	"bufio"
	"fmt"
	"io"

	"github.com/filecoin-project/venus/internal/pkg/encoding"
)

// ErrMessageTooLarge is returned when reading too big of a message
var ErrMessageTooLarge = fmt.Errorf("attempted to read a message larger than the limit")

// MsgReader is a cbor message reader
type MsgReader struct {
	br *bufio.Reader
}

// NewMsgReader returns a new MsgReader
func NewMsgReader(r io.Reader) *MsgReader {
	return &MsgReader{
		br: bufio.NewReader(r),
	}
}

// ReadMsg reads a cbor message into the given object
func (mr *MsgReader) ReadMsg(i interface{}) error {
	return encoding.StreamDecode(mr.br, i)
}
