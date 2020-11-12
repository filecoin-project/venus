package exchange

import (
	"io"

	"github.com/filecoin-project/venus/internal/pkg/encoding"
)

// WriteCborRPC with encode an object to cbor, opting for fast path if possible
// and then write it into the given io.Writer
func WriteCborRPC(w io.Writer, obj interface{}) error {
	data, err := encoding.Encode(obj)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// ReadCborRPC will read an object from the given io.Reader
// opting for fast path if possible
func ReadCborRPC(r io.Reader, out interface{}) error {
	return encoding.StreamDecode(r, out)
}
