package encoding

import (
	cbor "github.com/ipfs/go-ipld-cbor"
)

// CborEncoder is an object encoder that encodes objects based on the CBOR standard.
type IpldCborEncoder struct {
	raw []byte
}

// CborDecoder is an object decoder that decodes objects based on the CBOR standard.
type IpldCborDecoder struct {
	raw []byte
}

// RegisterCborType registers a type for Cbor encoding/decoding
func RegisterIpldCborType(i interface{}) {
	cbor.RegisterCborType(i)
}

//
// CborEncoder
//

func (encoder *IpldCborEncoder) EncodeObject(obj Encodable) error {
	var err error

	encoder.raw, err = cbor.DumpObject(obj)
	if err != nil {
		return err
	}

	return nil
}

func (encoder IpldCborEncoder) IntoBytes() []byte {
	return encoder.raw
}

//
// CborDecoder
//

func (decoder *IpldCborDecoder) SetBytes(raw []byte) {
	decoder.raw = raw
}

func (decoder IpldCborDecoder) DecodeObject(obj Decodable) error {
	err := cbor.DecodeInto(decoder.raw, obj)
	if err != nil {
		return err
	}

	return nil
}
