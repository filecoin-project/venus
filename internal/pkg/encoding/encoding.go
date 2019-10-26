package encoding

import (
	"fmt"
	"reflect"
)

// Encodable represents types that can be encoded using this library.
type Encodable interface {
	Encode(encoder Encoder) error
}

// Decodable represents types that can be decoded using this library.
type Decodable interface {
	Decode(decoder Decoder) error
}

// Encoder represents types that can encode values.
type Encoder interface {
	// EncodeUint encodes a uint.
	EncodeUint(obj uint) error
	// EncodeUint8 encodes a uint8.
	EncodeUint8(obj uint8) error
	// EncodeUint16 encodes a uint16.
	EncodeUint16(obj uint16) error
	// EncodeUint32 encodes a uint32.
	EncodeUint32(obj uint32) error
	// EncodeUint64 encodes a uint64.
	EncodeUint64(obj uint64) error
	// EncodeInt encodes a int8.
	EncodeInt(obj int) error
	// EncodeInt8 encodes a int8.
	EncodeInt8(obj int8) error
	// EncodeInt16 encodes a int16.
	EncodeInt16(obj int16) error
	// EncodeInt32 encodes a int32.
	EncodeInt32(obj int32) error
	// EncodeInt64 encodes a int64.
	EncodeInt64(obj int64) error
	// EncodeBool encodes a bool.
	EncodeBool(obj bool) error
	// EncodeString encodes a string.
	EncodeString(obj string) error
	// EncodeArray encodes an array.
	EncodeArray(obj interface{}) error
	// EncodeMap encodes a map.
	EncodeMap(obj interface{}) error
	// EncodeStruct encodes a struct.
	EncodeStruct(obj interface{}) error
	// Bytes returns the encoded bytes.
	Bytes() []byte
}

// Decoder represents types that can decode values.
type Decoder interface {
	// DecodeValue decodes a primitive value.
	DecodeValue(obj interface{}) error
	// DecodeArray decodes an array.
	DecodeArray(obj interface{}) error
	// DecodeMap decodes a map.
	DecodeMap(obj interface{}) error
	// DecodeStruct decodes a struct.
	DecodeStruct(obj interface{}) error
}

type defaultEncoder = IpldCborEncoder
type defaultDecoder = IpldCborDecoder

// Encode encodes an object, returning a byte array.
func Encode(obj interface{}) ([]byte, error) {
	var encoder Encoder = &defaultEncoder{}
	return encode(obj, reflect.ValueOf(obj), encoder)
}

// EncodeWith encodes an object using the encoder provided returning a byte array.
func EncodeWith(obj interface{}, encoder Encoder) ([]byte, error) {
	return encode(obj, reflect.ValueOf(obj), encoder)
}

func encode(obj interface{}, v reflect.Value, encoder Encoder) ([]byte, error) {
	var err error

	// if `Encodable`, we are done
	if encodable, ok := obj.(Encodable); ok {
		if err = encodable.Encode(encoder); err != nil {
			return nil, err
		}
		return encoder.Bytes(), nil
	}

	// Note: this -> (v.Convert(reflect.TypeOf(uint64(0))).Interface().(uint64))
	//       is because doing `obj.(uint64)` blows up on `type foo uint64`

	switch v.Kind() {
	case reflect.Uint:
		err = encoder.EncodeUint(v.Convert(reflect.TypeOf(uint(0))).Interface().(uint))
	case reflect.Uint8:
		err = encoder.EncodeUint8(v.Convert(reflect.TypeOf(uint8(0))).Interface().(uint8))
	case reflect.Uint16:
		err = encoder.EncodeUint16(v.Convert(reflect.TypeOf(uint16(0))).Interface().(uint16))
	case reflect.Uint32:
		err = encoder.EncodeUint32(v.Convert(reflect.TypeOf(uint32(0))).Interface().(uint32))
	case reflect.Uint64:
		err = encoder.EncodeUint64(v.Convert(reflect.TypeOf(uint64(0))).Interface().(uint64))
	case reflect.Int:
		err = encoder.EncodeInt(v.Convert(reflect.TypeOf(int(0))).Interface().(int))
	case reflect.Int8:
		err = encoder.EncodeInt8(v.Convert(reflect.TypeOf(int8(0))).Interface().(int8))
	case reflect.Int16:
		err = encoder.EncodeInt16(v.Convert(reflect.TypeOf(int16(0))).Interface().(int16))
	case reflect.Int32:
		err = encoder.EncodeInt32(v.Convert(reflect.TypeOf(int32(0))).Interface().(int32))
	case reflect.Int64:
		err = encoder.EncodeInt64(v.Convert(reflect.TypeOf(int64(0))).Interface().(int64))
	case reflect.Bool:
		err = encoder.EncodeBool(v.Convert(reflect.TypeOf(false)).Interface().(bool))
	case reflect.String:
		err = encoder.EncodeString(v.Convert(reflect.TypeOf("")).Interface().(string))
	case reflect.Slice:
		err = encoder.EncodeArray(obj)
	case reflect.Map:
		err = encoder.EncodeMap(obj)
	case reflect.Struct:
		err = encoder.EncodeStruct(obj)
	case reflect.Ptr:
		if v.IsNil() {
			t := v.Type()
			nv := reflect.New(t.Elem())
			return encode(obj, nv, encoder)
		}
		// navigate the pointer and check the underlying type
		return encode(obj, reflect.Indirect(v), encoder)
	case reflect.Interface:
		// navigate the interface and check the underlying type
		return encode(obj, v.Elem(), encoder)
	default:
		return nil, fmt.Errorf("unsupported type for encoding: %T", obj)
	}

	if err != nil {
		return nil, err
	}

	return encoder.Bytes(), nil
}

// DecodeWith decodes a decodable type, and populates a pointer to the type.
func DecodeWith(obj interface{}, decoder Decoder) error {
	return decode(obj, reflect.ValueOf(obj), decoder)
}

// Decode decodes a decodable type, and populates a pointer to the type.
func Decode(raw []byte, obj interface{}) error {
	var decoder Decoder = &defaultDecoder{
		raw: raw,
	}

	return decode(obj, reflect.ValueOf(obj), decoder)
}

func decode(obj interface{}, v reflect.Value, decoder Decoder) error {
	var err error

	// if `Decodable`, we are done
	if decodable, ok := obj.(Decodable); ok {
		if err = decodable.Decode(decoder); err != nil {
			return err
		}
		return nil
	}
	k := v.Kind()
	switch k {
	case reflect.Uint:
		return decoder.DecodeValue(obj)
	case reflect.Uint8:
		return decoder.DecodeValue(obj)
	case reflect.Uint16:
		return decoder.DecodeValue(obj)
	case reflect.Uint32:
		return decoder.DecodeValue(obj)
	case reflect.Uint64:
		return decoder.DecodeValue(obj)
	// case uint128: TODO: Big uint?
	case reflect.Int:
		return decoder.DecodeValue(obj)
	case reflect.Int8:
		return decoder.DecodeValue(obj)
	case reflect.Int16:
		return decoder.DecodeValue(obj)
	case reflect.Int32:
		return decoder.DecodeValue(obj)
	case reflect.Int64:
		// case int128: TODO: Big int?
		return decoder.DecodeValue(obj)
	case reflect.Bool:
		return decoder.DecodeValue(obj)
	case reflect.String:
		return decoder.DecodeValue(obj)
	case reflect.Slice:
		return decoder.DecodeArray(obj)
	case reflect.Map:
		return decoder.DecodeMap(obj)
	case reflect.Struct:
		return decoder.DecodeStruct(obj)
	case reflect.Ptr:
		if v.IsNil() {
			t := v.Type()
			nv := reflect.New(t.Elem())
			return decode(obj, nv, decoder)
		}
		return decode(obj, reflect.Indirect(v), decoder)
	case reflect.Interface:
		return decode(obj, v.Elem(), decoder)
	default:
		return fmt.Errorf("unsupported type for decoding: %T, kind: %v", obj, k)
	}
}
