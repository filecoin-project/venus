package encoding

import (
	"bytes"
	"testing"

	"gotest.tools/assert"
)

func TestIpldCborEncodingOutput(t *testing.T) {
	var original = &Point{X: 8, Y: 3}
	var encoder = IpldCborEncoder{}

	err := encoder.EncodeObject(original)
	assert.NilError(t, err)

	output := encoder.IntoBytes()

	var expected = []byte{162, 97, 120, 8, 97, 121, 3}
	assert.Assert(t, bytes.Equal(output, expected))
}

func TestIpldCborDecodingOutput(t *testing.T) {
	var input = []byte{162, 97, 120, 8, 97, 121, 3}

	var decoder = &IpldCborDecoder{}
	decoder.SetBytes(input)

	var output = Point{}
	err := decoder.DecodeObject(&output)
	assert.NilError(t, err)

	var expected = Point{X: 8, Y: 3}
	assert.Equal(t, output, expected)
}

func TestIpldCborDecodingFromWhyOutput(t *testing.T) {
	var input = []byte{130, 8, 3}

	var decoder = &IpldCborDecoder{}
	decoder.SetBytes(input)

	var output = Point{}
	err := decoder.DecodeObject(&output)
	assert.NilError(t, err)

	var expected = Point{X: 8, Y: 3}
	assert.Equal(t, output, expected)
}
