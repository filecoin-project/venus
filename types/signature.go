package types

import (
	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"
	"gx/ipfs/QmcrriCMhjb5ZWzmPNxmP53px47tSPcXBNaMtLdgcKFJYk/refmt/obj/atlas"
)

// Signature is the result of a cryptographic sign operation.
type Signature []byte

func init() {
	// This is a workaround for: https://github.com/polydawn/refmt/issues/27.
	cbor.RegisterCborType(atlas.BuildEntry(Signature{}).Transform().
		TransformMarshal(atlas.MakeMarshalTransformFunc(
			func(s Signature) (interface{}, error) {
				if s == nil {
					return nil, nil
				}
				return []byte(s), nil
			})).
		TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
			func(s interface{}) (Signature, error) {
				if s == nil {
					return Signature(nil), nil
				}
				return Signature(s.([]byte)), nil
			})).
		Complete())
}
