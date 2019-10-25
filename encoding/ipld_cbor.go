package encoding

import (
	"bytes"

	cbor "github.com/ipfs/go-ipld-cbor"
)

// IpldCborEncoder is an object encoder that encodes objects based on the CBOR standard.
type IpldCborEncoder struct {
	b bytes.Buffer
}

// IpldCborDecoder is an object decoder that decodes objects based on the CBOR standard.
type IpldCborDecoder struct {
	raw []byte
}

// NewIpldCborEncoder creates a new `IpldCborEncoder`.
func NewIpldCborEncoder() IpldCborEncoder {
	return IpldCborEncoder{}
}

// NewIpldCborDecoder creates a new `IpldCborDecoder`.
func NewIpldCborDecoder(b []byte) IpldCborDecoder {
	return IpldCborDecoder{
		raw: b,
	}
}

// RegisterIpldCborType registers a type for Cbor encoding/decoding.
func RegisterIpldCborType(i interface{}) {
	cbor.RegisterCborType(i)
}

//
// IpldCborEncoder
//

// EncodeUint encodes a uint.
func (encoder *IpldCborEncoder) EncodeUint(obj uint) error {
	return encoder.encodeCbor(obj)
}

// EncodeUint8 encodes a uint8.
func (encoder *IpldCborEncoder) EncodeUint8(obj uint8) error {
	return encoder.encodeCbor(obj)
}

// EncodeUint16 encodes a uint16.
func (encoder *IpldCborEncoder) EncodeUint16(obj uint16) error {
	return encoder.encodeCbor(obj)
}

// EncodeUint32 encodes a uint32.
func (encoder *IpldCborEncoder) EncodeUint32(obj uint32) error {
	return encoder.encodeCbor(obj)
}

// EncodeUint64 encodes a uint64.
func (encoder *IpldCborEncoder) EncodeUint64(obj uint64) error {
	return encoder.encodeCbor(obj)
}

// EncodeInt encodes a int.
func (encoder *IpldCborEncoder) EncodeInt(obj int) error {
	return encoder.encodeCbor(obj)
}

// EncodeInt8 encodes a int8.
func (encoder *IpldCborEncoder) EncodeInt8(obj int8) error {
	return encoder.encodeCbor(obj)
}

// EncodeInt16 encodes a int16.
func (encoder *IpldCborEncoder) EncodeInt16(obj int16) error {
	return encoder.encodeCbor(obj)
}

// EncodeInt32 encodes a int32.
func (encoder *IpldCborEncoder) EncodeInt32(obj int32) error {
	return encoder.encodeCbor(obj)
}

// EncodeInt64 encodes a int64.
func (encoder *IpldCborEncoder) EncodeInt64(obj int64) error {
	return encoder.encodeCbor(obj)
}

// EncodeBool encodes a bool.
func (encoder *IpldCborEncoder) EncodeBool(obj bool) error {
	return encoder.encodeCbor(obj)
}

// EncodeString encodes a string.
func (encoder *IpldCborEncoder) EncodeString(obj string) error {
	return encoder.encodeCbor(obj)
}

// EncodeArray encodes an array.
func (encoder *IpldCborEncoder) EncodeArray(obj interface{}) error {
	return encoder.encodeCbor(obj)
}

// EncodeMap encodes a map.
func (encoder *IpldCborEncoder) EncodeMap(obj interface{}) error {
	return encoder.encodeCbor(obj)
}

// EncodeStruct encodes a struct.
func (encoder *IpldCborEncoder) EncodeStruct(obj interface{}) error {
	return encoder.encodeCbor(obj)
}

// Bytes returns the encoded bytes.
func (encoder IpldCborEncoder) Bytes() []byte {
	return encoder.b.Bytes()
}

func (encoder *IpldCborEncoder) encodeCbor(obj interface{}) error {
	// get cbor encoded bytes
	raw, err := cbor.DumpObject(obj)
	if err != nil {
		return err
	}

	// write to buffer
	encoder.b.Write(raw)

	return nil
}

//
// IpldCborDecoder
//

// DecodeValue encodes an array.
func (decoder *IpldCborDecoder) DecodeValue(obj interface{}) error {
	return decoder.decodeCbor(obj)
}

// DecodeArray encodes an array.
func (decoder *IpldCborDecoder) DecodeArray(obj interface{}) error {
	return decoder.decodeCbor(obj)
}

// DecodeMap encodes a map.
func (decoder *IpldCborDecoder) DecodeMap(obj interface{}) error {
	return decoder.decodeCbor(obj)
}

// DecodeStruct encodes a uint64.
func (decoder *IpldCborDecoder) DecodeStruct(obj interface{}) error {
	return decoder.decodeCbor(obj)
}

func (decoder *IpldCborDecoder) decodeCbor(obj interface{}) error {
	// decode the bytes into a cbor object
	if err := cbor.DecodeInto(decoder.raw, obj); err != nil {
		return err
	}
	// reset the bytes, nothing left with CBOR
	decoder.raw = nil

	return nil
}
