package bytesink

import "io"

// ByteSink represents a location to which bytes can be written. The ByteSink
// should be closed after all bytes have been written.
type ByteSink interface {
	io.Writer
	io.Closer
	Open() error
	ID() string
}
