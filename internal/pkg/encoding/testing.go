package encoding

import "reflect"

func init() {
	RegisterIpldCborType(defaultPoint{})
	RegisterIpldCborType(customPoint{})
}

type defaultPoint struct {
	X uint64
	Y uint64
}

type testEncoder struct {
	calls   []string
	lastobj interface{}
}

type testDecoder struct {
	calls   []string
	decoded interface{}
}

func newTestEncoder() testEncoder {
	return testEncoder{
		calls: []string{},
	}
}

func newTestDecoder() testDecoder {
	return testDecoder{
		calls: []string{},
	}
}

// EncodeUint encodes a uint.
func (encoder *testEncoder) EncodeUint(obj uint) error {
	encoder.calls = append(encoder.calls, "EncodeUint")
	encoder.lastobj = obj
	return nil
}

// EncodeUint8 encodes a uint8.
func (encoder *testEncoder) EncodeUint8(obj uint8) error {
	encoder.calls = append(encoder.calls, "EncodeUint8")
	encoder.lastobj = obj
	return nil
}

// EncodeUint16 encodes a uint16.
func (encoder *testEncoder) EncodeUint16(obj uint16) error {
	encoder.calls = append(encoder.calls, "EncodeUint16")
	encoder.lastobj = obj
	return nil
}

// EncodeUint32 encodes a uint32.
func (encoder *testEncoder) EncodeUint32(obj uint32) error {
	encoder.calls = append(encoder.calls, "EncodeUint32")
	encoder.lastobj = obj
	return nil
}

// EncodeUint64 encodes a uint64.
func (encoder *testEncoder) EncodeUint64(obj uint64) error {
	encoder.calls = append(encoder.calls, "EncodeUint64")
	encoder.lastobj = obj
	return nil
}

// EncodeInt encodes a int.
func (encoder *testEncoder) EncodeInt(obj int) error {
	encoder.calls = append(encoder.calls, "EncodeInt")
	encoder.lastobj = obj
	return nil
}

// EncodeInt8 encodes a int8.
func (encoder *testEncoder) EncodeInt8(obj int8) error {
	encoder.calls = append(encoder.calls, "EncodeInt8")
	encoder.lastobj = obj
	return nil
}

// EncodeInt16 encodes a int16.
func (encoder *testEncoder) EncodeInt16(obj int16) error {
	encoder.calls = append(encoder.calls, "EncodeInt16")
	encoder.lastobj = obj
	return nil
}

// EncodeInt32 encodes a int32.
func (encoder *testEncoder) EncodeInt32(obj int32) error {
	encoder.calls = append(encoder.calls, "EncodeInt32")
	encoder.lastobj = obj
	return nil
}

// EncodeInt64 encodes a int64.
func (encoder *testEncoder) EncodeInt64(obj int64) error {
	encoder.calls = append(encoder.calls, "EncodeInt64")
	encoder.lastobj = obj
	return nil
}

// EncodeBoolean encodes a bool.
func (encoder *testEncoder) EncodeBool(obj bool) error {
	encoder.calls = append(encoder.calls, "EncodeBool")
	encoder.lastobj = obj
	return nil
}

// EncodeString encodes a string.
func (encoder *testEncoder) EncodeString(obj string) error {
	encoder.calls = append(encoder.calls, "EncodeString")
	encoder.lastobj = obj
	return nil
}

// EncodeArray encodes an array.
func (encoder *testEncoder) EncodeArray(obj interface{}) error {
	encoder.calls = append(encoder.calls, "EncodeArray")
	encoder.lastobj = obj
	return nil
}

// EncodeMap encodes a map.
func (encoder *testEncoder) EncodeMap(obj interface{}) error {
	encoder.calls = append(encoder.calls, "EncodeMap")
	encoder.lastobj = obj
	return nil
}

// EncodeStruct encodes a uint64.
func (encoder *testEncoder) EncodeStruct(obj interface{}) error {
	encoder.calls = append(encoder.calls, "EncodeStruct")
	encoder.lastobj = obj
	return nil
}

// Bytes returns the encoded bytes.
func (encoder testEncoder) Bytes() []byte {
	return []byte{1, 2, 3}
}

// DecodeValue encodes an array.
func (decoder *testDecoder) DecodeValue(obj interface{}) error {
	decoder.calls = append(decoder.calls, "DecodeValue")
	set(reflect.ValueOf(obj), decoder.decoded)
	return nil
}

// DecodeArray encodes an array.
func (decoder *testDecoder) DecodeArray(obj interface{}) error {
	decoder.calls = append(decoder.calls, "DecodeArray")
	set(reflect.ValueOf(obj), decoder.decoded)
	return nil
}

// DecodeMap encodes a map.
func (decoder *testDecoder) DecodeMap(obj interface{}) error {
	decoder.calls = append(decoder.calls, "DecodeMap")
	set(reflect.ValueOf(obj), decoder.decoded)
	return nil
}

// EncodeStruct encodes a uint64.
func (decoder *testDecoder) DecodeStruct(obj interface{}) error {
	decoder.calls = append(decoder.calls, "DecodeStruct")
	set(reflect.ValueOf(obj), decoder.decoded)
	return nil
}

func set(v reflect.Value, to interface{}) {
	switch v.Kind() {
	case reflect.Interface:
		v.Elem().Set(reflect.ValueOf(to))
	case reflect.Ptr:
		if v.IsNil() {
			// Note: we need to figure out how to set things like a pointer to a nil pointer
			panic("not supported")
		} else {
			set(reflect.Indirect(v), to)
		}
	default:
		v.Set(reflect.ValueOf(to))
	}
}

type customPoint struct {
	X uint64
	Y uint64
}

func (p customPoint) Encode(encoder Encoder) error {
	if err := encoder.EncodeArray([]uint64{p.X, p.Y}); err != nil {
		return err
	}

	return nil
}

func (p *customPoint) Decode(decoder Decoder) error {
	decoded := []uint64{}
	if err := decoder.DecodeArray(&decoded); err != nil {
		return err
	}
	p.X = decoded[0]
	p.Y = decoded[1]
	return nil
}
