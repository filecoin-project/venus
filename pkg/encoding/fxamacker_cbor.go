package encoding

import (
	"bytes"
	"io"

	cbor "github.com/fxamacker/cbor/v2"
)

// FxamackerCborEncoder is an object encoder that encodes objects based on the CBOR standard.
type FxamackerCborEncoder struct {
	b bytes.Buffer
}

// FxamackerCborDecoder is an object decoder that decodes objects based on the CBOR standard.
type FxamackerCborDecoder struct {
	raw []byte
}

// NewFxamackerCborEncoder creates a new `FxamackerCborEncoder`.
func NewFxamackerCborEncoder() FxamackerCborEncoder {
	return FxamackerCborEncoder{}
}

// NewFxamackerCborDecoder creates a new `FxamackerCborDecoder`.
func NewFxamackerCborDecoder(b []byte) FxamackerCborDecoder {
	return FxamackerCborDecoder{
		raw: b,
	}
}

// FxamackerNewStreamDecoder initializes a new fxamacker cbor stream decoder
var FxamackerNewStreamDecoder = cbor.NewDecoder

//
// FxamackerCborEncoder
//

// EncodeUint encodes a uint.
func (encoder *FxamackerCborEncoder) EncodeUint(obj uint) error {
	return encoder.encodeCbor(obj)
}

// EncodeUint8 encodes a uint8.
func (encoder *FxamackerCborEncoder) EncodeUint8(obj uint8) error {
	return encoder.encodeCbor(obj)
}

// EncodeUint16 encodes a uint16.
func (encoder *FxamackerCborEncoder) EncodeUint16(obj uint16) error {
	return encoder.encodeCbor(obj)
}

// EncodeUint32 encodes a uint32.
func (encoder *FxamackerCborEncoder) EncodeUint32(obj uint32) error {
	return encoder.encodeCbor(obj)
}

// EncodeUint64 encodes a uint64.
func (encoder *FxamackerCborEncoder) EncodeUint64(obj uint64) error {
	return encoder.encodeCbor(obj)
}

// EncodeInt encodes a int.
func (encoder *FxamackerCborEncoder) EncodeInt(obj int) error {
	return encoder.encodeCbor(obj)
}

// EncodeInt8 encodes a int8.
func (encoder *FxamackerCborEncoder) EncodeInt8(obj int8) error {
	return encoder.encodeCbor(obj)
}

// EncodeInt16 encodes a int16.
func (encoder *FxamackerCborEncoder) EncodeInt16(obj int16) error {
	return encoder.encodeCbor(obj)
}

// EncodeInt32 encodes a int32.
func (encoder *FxamackerCborEncoder) EncodeInt32(obj int32) error {
	return encoder.encodeCbor(obj)
}

// EncodeInt64 encodes a int64.
func (encoder *FxamackerCborEncoder) EncodeInt64(obj int64) error {
	return encoder.encodeCbor(obj)
}

// EncodeBool encodes a bool.
func (encoder *FxamackerCborEncoder) EncodeBool(obj bool) error {
	return encoder.encodeCbor(obj)
}

// EncodeString encodes a string.
func (encoder *FxamackerCborEncoder) EncodeString(obj string) error {
	return encoder.encodeCbor(obj)
}

// EncodeArray encodes an array.
func (encoder *FxamackerCborEncoder) EncodeArray(obj interface{}) error {
	return encoder.encodeCbor(obj)
}

// EncodeMap encodes a map.
func (encoder *FxamackerCborEncoder) EncodeMap(obj interface{}) error {
	return encoder.encodeCbor(obj)
}

// EncodeStruct encodes a struct.
func (encoder *FxamackerCborEncoder) EncodeStruct(obj interface{}) error {
	return encoder.encodeCbor(obj)
}

// Bytes returns the encoded bytes.
func (encoder FxamackerCborEncoder) Bytes() []byte {
	return encoder.b.Bytes()
}

func (encoder *FxamackerCborEncoder) encodeCbor(obj interface{}) error {
	// check for object implementing cborMarshallerStreamed
	if m, ok := obj.(cborMarshalerStreamed); ok {
		return m.MarshalCBOR(&encoder.b)
	}

	// get cbor encoded bytes
	raw, err := cbor.Marshal(obj)
	if err != nil {
		return err
	}

	// write to buffer
	encoder.b.Write(raw)

	return nil
}

//
// FxamackerCborDecoder
//

// DecodeValue decodes an primitive value.
func (decoder *FxamackerCborDecoder) DecodeValue(obj interface{}) error {
	return decoder.decodeCbor(obj)
}

// DecodeArray decodes an array.
func (decoder *FxamackerCborDecoder) DecodeArray(obj interface{}) error {
	return decoder.decodeCbor(obj)
}

// DecodeMap encodes a map.
func (decoder *FxamackerCborDecoder) DecodeMap(obj interface{}) error {
	return decoder.decodeCbor(obj)
}

// DecodeStruct decodes a struct.
func (decoder *FxamackerCborDecoder) DecodeStruct(obj interface{}) error {

	return decoder.decodeCbor(obj)
}

func (decoder *FxamackerCborDecoder) decodeCbor(obj interface{}) error {
	// check for object implementing cborUnmarshallerStreamed
	if u, ok := obj.(cborUnmarshalerStreamed); ok {
		return u.UnmarshalCBOR(bytes.NewBuffer(decoder.raw))
	}

	// decode the bytes into a cbor object
	if err := cbor.Unmarshal(decoder.raw, obj); err != nil {
		return err
	}
	// reset the bytes, nothing left with CBOR
	decoder.raw = nil

	return nil
}

type cborUnmarshalerStreamed interface {
	UnmarshalCBOR(io.Reader) error
}

type cborMarshalerStreamed interface {
	MarshalCBOR(io.Writer) error
}
