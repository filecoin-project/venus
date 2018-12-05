package types

import (
	cbor "gx/ipfs/QmRoARq3nkUb13HSKZGepCZSWe5GrVPwx7xURJGZ7KWv9V/go-ipld-cbor"
	"gx/ipfs/QmfWqohMtbivn5NRJvtrLzCW3EU4QmoLvVNtmvo9vbdtVA/refmt/obj/atlas"
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
