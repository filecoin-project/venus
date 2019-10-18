package encoding

import (
	"bytes"

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

func (encoder *WhyCborEncoder) EncodeObject(obj Encodable) error {
	cborobj, ok := obj.(cbg.CBORMarshaler)
	if !ok {
		// XXX: return error
		return nil
	}
	return cborobj.MarshalCBOR(encoder.b)
}

func (encoder WhyCborEncoder) IntoBytes() []byte {
	return encoder.b.Bytes()
}

//
// CborDecoder
//

func (decoder *WhyCborDecoder) SetBytes(raw []byte) {
	decoder.b = bytes.NewBuffer(raw)
}

func (decoder WhyCborDecoder) DecodeObject(obj Decodable) error {
	cborobj, ok := obj.(cbg.CBORUnmarshaler)
	if !ok {
		// XXX: return error
		return nil
	}
	return cborobj.UnmarshalCBOR(decoder.b)
}
