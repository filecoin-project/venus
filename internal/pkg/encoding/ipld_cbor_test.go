package encoding

import (
	"bytes"
	"reflect"
	"testing"

	"gotest.tools/assert"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestIpldCborEncodingEncodeStruct(t *testing.T) {
	tf.UnitTest(t)

	var original = &defaultPoint{X: 8, Y: 3}
	var encoder = IpldCborEncoder{}

	err := encoder.EncodeStruct(original)
	assert.NilError(t, err)

	output := encoder.Bytes()

	var expected = []byte{162, 97, 120, 8, 97, 121, 3}
	assert.Assert(t, bytes.Equal(output, expected))
}

func TestIpldCborDecodingDecodeStruct(t *testing.T) {
	tf.UnitTest(t)

	var input = []byte{162, 97, 120, 8, 97, 121, 3}

	var decoder = NewIpldCborDecoder(input)

	var output = defaultPoint{}
	err := decoder.DecodeStruct(&output)
	assert.NilError(t, err)

	var expected = defaultPoint{X: 8, Y: 3}
	assert.Equal(t, output, expected)
}

func TestIpldCborEncodeDecodeIsClosed(t *testing.T) {
	tf.UnitTest(t)

	original := defaultPoint{X: 8, Y: 3}

	raw, err := Encode(original)
	assert.NilError(t, err)

	decoded := defaultPoint{}

	err = Decode(raw, &decoded)
	assert.NilError(t, err)

	assert.Assert(t, reflect.DeepEqual(original, decoded))
}

func TestIpldCborCustomEncoding(t *testing.T) {
	tf.UnitTest(t)

	original := customPoint{X: 8, Y: 3}

	raw, err := Encode(original)
	assert.NilError(t, err)

	var expected = []byte{130, 8, 3}
	assert.Assert(t, bytes.Equal(raw, expected))
}

func TestIpldCborCustomDecoding(t *testing.T) {
	tf.UnitTest(t)

	var input = []byte{130, 8, 3}

	var output = customPoint{}
	err := Decode(input, &output)
	assert.NilError(t, err)

	var expected = customPoint{X: 8, Y: 3}
	assert.Equal(t, output, expected)
}

type wrapper uint64

func TestIpldCborNewTypeEncoding(t *testing.T) {
	tf.UnitTest(t)
	var original = wrapper(873)
	var encoder = IpldCborEncoder{}

	output, err := EncodeWith(original, &encoder)
	assert.NilError(t, err)

	var expected = []byte{25, 3, 105}
	assert.Assert(t, bytes.Equal(output, expected))
}

func TestIpldCborNewTypeDecoding(t *testing.T) {
	tf.UnitTest(t)

	var input = []byte{25, 3, 105}
	var decoder = NewIpldCborDecoder(input)

	var output = wrapper(0)
	err := DecodeWith(&output, &decoder)
	assert.NilError(t, err)

	var expected = wrapper(873)
	assert.Equal(t, output, expected)
}
