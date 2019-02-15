package types

import (
	"gx/ipfs/QmNScbpMAm3r2D25kmfQ43JCbQ8QCtai4V4DNz5ebuXUuZ/refmt/obj/atlas"
	cbor "gx/ipfs/QmRZxJ7oybgnnwriuRub9JXp5YdFM9wiGSyRq38QC7swpS/go-ipld-cbor"
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
