package abi

import (
	"fmt"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/pkg/errors"
)

// EncodeValues encodes a set of abi values to raw bytes. Zero length arrays of
// values are normalized to nil
func EncodeValues(vals []*Value) ([]byte, error) {
	if len(vals) == 0 {
		return nil, nil
	}

	var arr [][]byte

	for _, val := range vals {
		data, err := val.Serialize()
		if err != nil {
			return nil, err
		}

		arr = append(arr, data)
	}

	return encoding.Encode(arr)
}

// DecodeValues decodes an array of abi values from the given buffer, using the
// provided type information.
func DecodeValues(data []byte, types []Type) ([]*Value, error) {
	if len(types) > 0 && len(data) == 0 {
		return nil, fmt.Errorf("expected %d parameters, but got 0", len(types))
	}

	if len(data) == 0 {
		return nil, nil
	}

	var arr [][]byte
	if err := encoding.Decode(data, &arr); err != nil {
		return nil, err
	}

	if len(arr) != len(types) {
		return nil, fmt.Errorf("expected %d parameters, but got %d", len(types), len(arr))
	}

	out := make([]*Value, 0, len(types))
	for i, t := range types {
		v, err := Deserialize(arr[i], t)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// ToEncodedValues converts from a list of go abi-compatible values to abi values and then encodes to raw bytes.
func ToEncodedValues(params ...interface{}) ([]byte, error) {
	vals, err := ToValues(params)
	if err != nil {
		return nil, errors.Wrap(err, "unable to convert params to values")
	}

	bytes, err := EncodeValues(vals)
	if err != nil {
		return nil, errors.Wrap(err, "unable to encode values")
	}

	return bytes, nil
}
