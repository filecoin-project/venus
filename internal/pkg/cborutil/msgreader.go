package cborutil

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
)

// MaxMessageSize is the maximum message size to read
const MaxMessageSize = 256 << 10

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

// ReadMsg reads a length delimited cbor message into the given object
func (mr *MsgReader) ReadMsg(i interface{}) error {
	l, err := binary.ReadUvarint(mr.br)
	if err != nil {
		return err
	}

	if l > MaxMessageSize {
		return ErrMessageTooLarge
	}

	// TODO: add a method in ipldcbor that accepts a reader so we can use the streaming unmarshalers
	// refmtcbor.NewUnmarshallerAtlased(mr.br, ipldcbor.Atlas)

	buf := make([]byte, l)
	_, err = io.ReadFull(mr.br, buf)
	if err != nil {
		return err
	}

	return encoding.Decode(buf, i)
}
