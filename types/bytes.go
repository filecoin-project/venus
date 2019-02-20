package types

import (
	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"
	"gx/ipfs/QmdBzoMxsBpojBfN1cv5GnKtB7sfYBMoLH7p9qSyEVYXcu/refmt/obj/atlas"
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
