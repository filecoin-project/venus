package cborutil

import (
	"bufio"
	"encoding/binary"
	"io"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
)

// MsgWriter is a length delimited cbor message writer
type MsgWriter struct {
	w *bufio.Writer
}

// NewMsgWriter returns a new MsgWriter
func NewMsgWriter(w io.Writer) *MsgWriter {
	return &MsgWriter{
		w: bufio.NewWriter(w),
	}
}

// WriteMsg writes the given object as a length delimited cbor blob
func (mw *MsgWriter) WriteMsg(i interface{}) error {
	// TODO: rewrite to be more memory efficient once we hook up the streaming
	// cbor interfaces
	data, err := encoding.Encode(i)
	if err != nil {
		return err
	}

	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(buf, uint64(len(data)))
	_, err = mw.w.Write(buf[:n])
	if err != nil {
		return err
	}

	_, err = mw.w.Write(data)
	if err != nil {
		return err
	}

	return mw.w.Flush()
}
