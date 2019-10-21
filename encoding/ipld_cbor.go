package encoding

import (
	cbor "github.com/ipfs/go-ipld-cbor"
)

// IpldCborEncoder is an object encoder that encodes objects based on the CBOR standard.
type IpldCborEncoder struct {
	raw []byte
}

// IpldCborDecoder is an object decoder that decodes objects based on the CBOR standard.
type IpldCborDecoder struct {
	raw []byte
}

// RegisterIpldCborType registers a type for Cbor encoding/decoding
func RegisterIpldCborType(i interface{}) {
	cbor.RegisterCborType(i)
}

//
// CborEncoder
//

// EncodeObject encodes an object.
func (encoder *IpldCborEncoder) EncodeObject(obj Encodable) error {
	var err error

	encoder.raw, err = cbor.DumpObject(obj)
	if err != nil {
		return err
	}

	return nil
}

// IntoBytes returns the encoded bytes.
func (encoder IpldCborEncoder) IntoBytes() []byte {
	return encoder.raw
}

//
// CborDecoder
//

// SetBytes sets the initializer internal bytes to match the input.
func (decoder *IpldCborDecoder) SetBytes(raw []byte) {
	decoder.raw = raw
}

// DecodeObject decodes an object.
func (decoder IpldCborDecoder) DecodeObject(obj Decodable) error {
	err := cbor.DecodeInto(decoder.raw, obj)
	if err != nil {
		return err
	}

	return nil
}
