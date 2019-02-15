package address

import (
	"sort"

	"gx/ipfs/QmNScbpMAm3r2D25kmfQ43JCbQ8QCtai4V4DNz5ebuXUuZ/refmt/obj/atlas"
	cbor "gx/ipfs/QmRZxJ7oybgnnwriuRub9JXp5YdFM9wiGSyRq38QC7swpS/go-ipld-cbor"
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
