package abi

// MustConvertParams abi encodes the given parameters into a byte array (or panics)
func MustConvertParams(params ...interface{}) []byte {
	vals, err := ToValues(params)
	if err != nil {
		panic(err)
	}

	out, err := EncodeValues(vals)
	if err != nil {
		panic(err)
	}
	return out
}
