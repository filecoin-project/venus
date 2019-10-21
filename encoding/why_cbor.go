package encoding

import (
	"bytes"
	"fmt"

	cbg "github.com/whyrusleeping/cbor-gen"
)

// WhyCborEncoder is an object encoder that encodes objects based on the CBOR standard.
type WhyCborEncoder struct {
	b *bytes.Buffer
}

// WhyCborDecoder is an object decoder that decodes objects based on the CBOR standard.
type WhyCborDecoder struct {
	b *bytes.Buffer
}

//
// CborEncoder
//

// EncodeObject encodes an object.
func (encoder *WhyCborEncoder) EncodeObject(obj Encodable) error {
	cborobj, ok := obj.(cbg.CBORMarshaler)
	if !ok {
		return fmt.Errorf("Object is not a CBORMarshaler")
	}
	return cborobj.MarshalCBOR(encoder.b)
}

// IntoBytes returns the encoded bytes.
func (encoder WhyCborEncoder) IntoBytes() []byte {
	return encoder.b.Bytes()
}

//
// CborDecoder
//

// SetBytes sets the initializer internal bytes to match the input.
func (decoder *WhyCborDecoder) SetBytes(raw []byte) {
	decoder.b = bytes.NewBuffer(raw)
}

// DecodeObject decodes an object.
func (decoder WhyCborDecoder) DecodeObject(obj Decodable) error {
	cborobj, ok := obj.(cbg.CBORUnmarshaler)
	if !ok {
		return fmt.Errorf("Object is not a CBORUnmarshaler")
	}
	return cborobj.UnmarshalCBOR(decoder.b)
}
