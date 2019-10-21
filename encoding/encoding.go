package encoding

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
	// EncodeObject encodes an object.
	EncodeObject(obj Encodable) error
	// IntoBytes returns the encoded bytes.
	IntoBytes() []byte
}

// Decoder represents types that can decode values.
type Decoder interface {
	// SetBytes sets the initializer internal bytes to match the input.
	SetBytes([]byte)
	// DecodeObject decodes an object.
	DecodeObject(obj Decodable) error
}

type defaultEncoder = IpldCborEncoder
type defaultDecoder = IpldCborDecoder

// Encode encodes an encodable type, returning a byte array.
func Encode(obj Encodable) ([]byte, error) {
	var encoder Encoder = &defaultEncoder{}

	err := obj.Encode(encoder)
	if err != nil {
		return nil, err
	}

	return encoder.IntoBytes(), nil
}

// Decode decodes a decodable type, and populates a pointer to the type.
func Decode(raw []byte, obj Decodable) error {
	var decoder Decoder = &defaultDecoder{}
	decoder.SetBytes(raw)
	return obj.Decode(decoder)
}
