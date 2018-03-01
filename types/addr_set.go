package types

import (
	"sort"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"
	"gx/ipfs/QmcrriCMhjb5ZWzmPNxmP53px47tSPcXBNaMtLdgcKFJYk/refmt/obj/atlas"
)

func init() {
	cbor.RegisterCborType(addrSetEntry)
}

// AddrSet is a set of addresses
type AddrSet map[Address]struct{}

var addrSetEntry = atlas.BuildEntry(AddrSet{}).Transform().
	TransformMarshal(atlas.MakeMarshalTransformFunc(
		func(s AddrSet) ([]string, error) {
			out := make([]string, 0, len(s))
			for k := range s {
				out = append(out, string(k))
			}

			sort.Strings(out)

			return out, nil
		})).
	TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
		func(vals []string) (AddrSet, error) {
			out := make(AddrSet)
			for _, val := range vals {
				out[Address(val)] = struct{}{}
			}
			return out, nil
		})).
	Complete()
