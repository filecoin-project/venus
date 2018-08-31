package actor

import (
	cbor "gx/ipfs/QmV6BQ6fFCf9eFHDuRxvguvqfKLZtZrxthgZvDfRCs4tMN/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/exec"
)

// MarshalStorage encodes the passed in data into bytes.
func MarshalStorage(in interface{}) ([]byte, error) {
	return cbor.DumpObject(in)
}

// UnmarshalStorage decodes the passed in bytes into the given object.
func UnmarshalStorage(raw []byte, to interface{}) error {
	return cbor.DecodeInto(raw, to)
}

// WithState is a helper method that makes dealing with storage serialization
// easier for implementors.
// It is designed to be used like:
//
// var st MyStorage
// ret, err := WithState(ctx, &st, func() (interface{}, error) {
//   fmt.Println("hey look, my storage is loaded: ", st)
//   return st.Thing, nil
// })
//
// Note that if 'f' returns an error, modifications to the storage are not
// saved.
func WithState(ctx exec.VMContext, st interface{}, f func() (interface{}, error)) (interface{}, error) {
	chunk, err := ctx.ReadStorage()
	if err != nil {
		return nil, err
	}

	if err := UnmarshalStorage(chunk, st); err != nil {
		return nil, err
	}

	ret, err := f()
	if err != nil {
		return nil, err
	}

	data, err := MarshalStorage(st)
	if err != nil {
		return nil, err
	}

	if err := ctx.WriteStorage(data); err != nil {
		return nil, err
	}

	return ret, nil
}
