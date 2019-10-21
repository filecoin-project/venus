package encoding

import (
	"bytes"
	"testing"

	"gotest.tools/assert"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

func TestIpldCborEncodingOutput(t *testing.T) {
	tf.UnitTest(t)

	var original = &Point{X: 8, Y: 3}
	var encoder = IpldCborEncoder{}

	err := encoder.EncodeObject(original)
	assert.NilError(t, err)

	output := encoder.IntoBytes()

	var expected = []byte{162, 97, 120, 8, 97, 121, 3}
	assert.Assert(t, bytes.Equal(output, expected))
}

func TestIpldCborDecodingOutput(t *testing.T) {
	tf.UnitTest(t)

	var input = []byte{162, 97, 120, 8, 97, 121, 3}

	var decoder = &IpldCborDecoder{}
	decoder.SetBytes(input)

	var output = Point{}
	err := decoder.DecodeObject(&output)
	assert.NilError(t, err)

	var expected = Point{X: 8, Y: 3}
	assert.Equal(t, output, expected)
}
