package data

// ReadWriter allows for efficiently reading and writing slices of data.
type ReadWriter interface {
	DataAt(offset, length uint64, cb func([]byte) error) error
}
