package types

import (
	cbor "gx/ipfs/QmRiRJhn427YVuufBEHofLreKWNw7P7BWNq86Sb9kzqdbd/go-ipld-cbor"
	"gx/ipfs/QmcrriCMhjb5ZWzmPNxmP53px47tSPcXBNaMtLdgcKFJYk/refmt/obj/atlas"
)

// Bytes is a workaround for: https://github.com/polydawn/refmt/issues/27.
// Any of our types who want a nillable byte slice can use this instead.
type Bytes []byte

func init() {
	cbor.RegisterCborType(atlas.BuildEntry(Bytes{}).Transform().
		TransformMarshal(atlas.MakeMarshalTransformFunc(
			func(b Bytes) (interface{}, error) {
				if b == nil {
					return nil, nil
				}
				return []byte(b), nil
			})).
		TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
			func(b interface{}) (Bytes, error) {
				if b == nil {
					return Bytes(nil), nil
				}
				return Bytes(b.([]byte)), nil
			})).
		Complete())
}
