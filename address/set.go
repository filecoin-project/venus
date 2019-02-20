package address

import (
	"sort"

	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"
	"gx/ipfs/QmdBzoMxsBpojBfN1cv5GnKtB7sfYBMoLH7p9qSyEVYXcu/refmt/obj/atlas"
)

func init() {
	cbor.RegisterCborType(addrSetEntry)
}

// Set is a set of addresses
type Set map[Address]struct{}

var addrSetEntry = atlas.BuildEntry(Set{}).Transform().
	TransformMarshal(atlas.MakeMarshalTransformFunc(
		func(s Set) ([]byte, error) {
			out := make([]string, 0, len(s))
			for k := range s {
				out = append(out, string(k.Bytes()))
			}

			sort.Strings(out)

			bytes := make([]byte, 0, len(out)*Length)
			for _, k := range out {
				bytes = append(bytes, []byte(k)...)
			}
			return bytes, nil
		})).
	TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
		func(vals []byte) (Set, error) {
			out := make(Set)
			for i := 0; i < len(vals); i += Length {
				end := i + Length
				if end > len(vals) {
					end = len(vals)
				}
				s, err := NewFromBytes(vals[i:end])
				if err != nil {
					return nil, err
				}
				out[s] = struct{}{}
			}
			return out, nil
		})).
	Complete()
