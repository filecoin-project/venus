package encoding

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"gotest.tools/assert"

	cbg "github.com/whyrusleeping/cbor-gen"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestWhyCborEncodingOutput(t *testing.T) {
	tf.UnitTest(t)

	var original = &customPoint{X: 8, Y: 3}
	var encoder = WhyCborEncoder{b: bytes.NewBuffer([]byte{})}

	err := encoder.EncodeObject(original)
	assert.NilError(t, err)

	output := encoder.IntoBytes()

	var expected = []byte{130, 8, 3}
	assert.Assert(t, bytes.Equal(output, expected))
}

func TestWhyCborDecodingOutput(t *testing.T) {
	tf.UnitTest(t)

	var input = []byte{130, 8, 3}

	var decoder = &WhyCborDecoder{}
	decoder.SetBytes(input)

	var output = customPoint{}
	err := decoder.DecodeObject(&output)
	assert.NilError(t, err)

	var expected = customPoint{X: 8, Y: 3}
	assert.Equal(t, output, expected)
}

func (t *customPoint) MarshalCBOR(w io.Writer) error {
	if _, err := w.Write([]byte{130}); err != nil {
		return err
	}

	// t.t.X (uint64)
	if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajUnsignedInt, t.X)); err != nil {
		return err
	}

	// t.t.Y (uint64)
	if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajUnsignedInt, t.Y)); err != nil {
		return err
	}
	return nil
}

func (t *customPoint) UnmarshalCBOR(br io.Reader) error {

	maj, extra, err := cbg.CborReadHeader(br)
	if err != nil {
		return err
	}
	if maj != cbg.MajArray {
		return fmt.Errorf("cbor input should be of type array")
	}

	if extra != 2 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.t.X (uint64)

	maj, extra, err = cbg.CborReadHeader(br)
	if err != nil {
		return err
	}
	if maj != cbg.MajUnsignedInt {
		return fmt.Errorf("wrong type for uint64 field")
	}
	t.X = extra
	// t.t.Y (uint64)

	maj, extra, err = cbg.CborReadHeader(br)
	if err != nil {
		return err
	}
	if maj != cbg.MajUnsignedInt {
		return fmt.Errorf("wrong type for uint64 field")
	}
	t.Y = extra
	return nil
}
