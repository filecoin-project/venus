package encoding

import (
	"testing"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"gotest.tools/assert"
)

func doTestForEncoding(t *testing.T, original interface{}, expectedcalls interface{}) {
	tf.UnitTest(t)

	encoder := newTestEncoder()

	out, err := EncodeWith(original, &encoder)
	assert.NilError(t, err)
	assert.DeepEqual(t, out, []byte{1, 2, 3})

	assert.DeepEqual(t, encoder.calls, expectedcalls)
	assert.DeepEqual(t, encoder.lastobj, original)
}

func TestEncodingForEncodeUint8(t *testing.T) {
	doTestForEncoding(t, uint8(21), []string{"EncodeUint8"})
}

func TestEncodingForEncodeUint16(t *testing.T) {
	doTestForEncoding(t, uint16(21), []string{"EncodeUint16"})
}

func TestEncodingForEncodeUint32(t *testing.T) {
	doTestForEncoding(t, uint32(21), []string{"EncodeUint32"})
}

func TestEncodingForEncodeUint64(t *testing.T) {
	doTestForEncoding(t, uint64(21), []string{"EncodeUint64"})
}

func TestEncodingForEncodeInt8(t *testing.T) {
	doTestForEncoding(t, int8(21), []string{"EncodeInt8"})
}

func TestEncodingForEncodeInt16(t *testing.T) {
	doTestForEncoding(t, int16(21), []string{"EncodeInt16"})
}

func TestEncodingForEncodeInt32(t *testing.T) {
	doTestForEncoding(t, int32(21), []string{"EncodeInt32"})
}

func TestEncodingForEncodeInt64(t *testing.T) {
	doTestForEncoding(t, int64(21), []string{"EncodeInt64"})

}

func TestEncodingForEncodeBool(t *testing.T) {
	doTestForEncoding(t, false, []string{"EncodeBool"})
}

func TestEncodingForEncodeString(t *testing.T) {
	doTestForEncoding(t, "hello", []string{"EncodeString"})
}

func TestEncodingForEncodeArray(t *testing.T) {
	doTestForEncoding(t, []uint64{6, 2, 8}, []string{"EncodeArray"})
}

func TestEncodingForEncodeMap(t *testing.T) {
	doTestForEncoding(t, map[string]uint64{"x": 6, "y": 8}, []string{"EncodeMap"})
}

func TestEncodingForEncodeStruct(t *testing.T) {
	doTestForEncoding(t, struct {
		X uint32
		Y byte
	}{X: 6, Y: 8}, []string{"EncodeStruct"})
}

func TestEncodingForPointer(t *testing.T) {
	obj := struct {
		X uint32
		Y byte
	}{X: 6, Y: 8}
	doTestForEncoding(t, &obj, []string{"EncodeStruct"})
}

func TestEncodingForEncodable(t *testing.T) {
	tf.UnitTest(t)

	encoder := newTestEncoder()
	original := customPoint{X: 6, Y: 8}

	out, err := EncodeWith(original, &encoder)
	assert.NilError(t, err)
	assert.DeepEqual(t, out, []byte{1, 2, 3})

	assert.DeepEqual(t, encoder.calls, []string{"EncodeArray"})
	assert.DeepEqual(t, encoder.lastobj, []uint64{6, 8})
}

func TestEncodingNil(t *testing.T) {
	var obj *defaultPoint
	doTestForEncoding(t, obj, []string{"EncodeStruct"})
}

func doTestForDecoding(t *testing.T, obj interface{}, expected interface{}, expectedcalls interface{}) {
	tf.UnitTest(t)

	decoder := newTestDecoder()
	// inject decoded value
	decoder.decoded = expected

	err := DecodeWith(obj, &decoder)
	assert.NilError(t, err)

	assert.DeepEqual(t, decoder.calls, expectedcalls)
}

func TestDecodingForDecodeArray(t *testing.T) {
	obj := []uint64{}
	expected := []uint64{54, 2, 2}
	doTestForDecoding(t, &obj, expected, []string{"DecodeArray"})
	assert.DeepEqual(t, obj, expected)
}

func TestDecodingForMap(t *testing.T) {
	obj := map[string]uint64{}
	expected := map[string]uint64{"x": 6, "y": 8}
	doTestForDecoding(t, &obj, expected, []string{"DecodeMap"})
	assert.DeepEqual(t, obj, expected)
}

func TestDecodingForStruct(t *testing.T) {
	obj := defaultPoint{}
	expected := defaultPoint{X: 8, Y: 4}
	doTestForDecoding(t, &obj, expected, []string{"DecodeStruct"})
	assert.DeepEqual(t, obj, expected)
}

func TestDecodingForDecodable(t *testing.T) {
	obj := customPoint{}
	expected := customPoint{X: 8, Y: 4}

	tf.UnitTest(t)

	decoder := newTestDecoder()
	// inject decoded value
	decoder.decoded = []uint64{8, 4}

	err := DecodeWith(&obj, &decoder)
	assert.NilError(t, err)

	assert.DeepEqual(t, decoder.calls, []string{"DecodeArray"})
	assert.DeepEqual(t, obj, expected)
}

func TestDecodingForDoublePointerStruct(t *testing.T) {
	obj := &defaultPoint{}
	expected := defaultPoint{X: 8, Y: 4}
	doTestForDecoding(t, &obj, expected, []string{"DecodeStruct"})
	assert.DeepEqual(t, obj, &expected)
}

func TestDecodingForDoublePointerMap(t *testing.T) {
	obj := &map[string]uint64{}
	expected := map[string]uint64{"x": 6, "y": 8}
	doTestForDecoding(t, &obj, expected, []string{"DecodeMap"})
	assert.DeepEqual(t, obj, &expected)
}

type testMode int

const testConstForMode = testMode(iota)

func TestDecodingForConstValue(t *testing.T) {
	obj := testConstForMode
	expected := testConstForMode
	doTestForDecoding(t, &obj, expected, []string{"DecodeValue"})
	assert.DeepEqual(t, obj, expected)
}

func TestDecodingOnNil(t *testing.T) {
	// Note: this is needed here for reasons.
	// (answers might be found in CI land)
	tf.UnitTest(t)
	defer mustPanic(t)

	var obj *map[string]uint64
	expected := map[string]uint64{}
	doTestForDecoding(t, &obj, expected, []string{"DecodeValue"})
	assert.DeepEqual(t, obj, expected)
}

func mustPanic(t *testing.T) {
	if r := recover(); r == nil {
		t.Fail()
	}
}
