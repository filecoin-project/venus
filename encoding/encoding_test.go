package encoding

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
)

type Point struct {
	X int32
	Y int32
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
	var original Encodable = &Point{X: 8, Y: 3}

	raw, err := Encode(original)
	require.NoError(t, err)

	var decoded Decodable = &Point{}

	err = Decode(raw, decoded)
	require.NoError(t, err)

	assert.Assert(t, reflect.DeepEqual(original, decoded))
}
