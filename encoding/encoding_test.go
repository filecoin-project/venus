package encoding

import (
	"reflect"
	"testing"

	"gotest.tools/assert"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

// Point is the testing point.
type Point struct {
	X uint64
	Y uint64
}

func init() {
	RegisterIpldCborType(Point{})
}

func (p Point) Encode(encoder Encoder) error {
	var err error

	if err = encoder.EncodeObject(p); err != nil {
		return err
	}

	return nil
}

func (p *Point) Decode(decoder Decoder) error {
	if err := decoder.DecodeObject(p); err != nil {
		return err
	}

	return nil
}

func TestEncodeDecodeIsClosed(t *testing.T) {
	tf.UnitTest(t)

	var original Encodable = &Point{X: 8, Y: 3}

	raw, err := Encode(original)
	assert.NilError(t, err)

	var decoded Decodable = &Point{}

	err = Decode(raw, decoded)
	assert.NilError(t, err)

	assert.Assert(t, reflect.DeepEqual(original, decoded))
}
