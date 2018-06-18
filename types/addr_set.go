package types

import (
	"sort"

	cbor "gx/ipfs/QmRiRJhn427YVuufBEHofLreKWNw7P7BWNq86Sb9kzqdbd/go-ipld-cbor"
	"gx/ipfs/QmcrriCMhjb5ZWzmPNxmP53px47tSPcXBNaMtLdgcKFJYk/refmt/obj/atlas"
)

func init() {
	cbor.RegisterCborType(addrSetEntry)
}

// AddrSet is a set of addresses
type AddrSet map[Address]struct{}

var addrSetEntry = atlas.BuildEntry(AddrSet{}).Transform().
	TransformMarshal(atlas.MakeMarshalTransformFunc(
		func(s AddrSet) ([]byte, error) {
			out := make([]string, 0, len(s))
			for k := range s {
				out = append(out, string(k.Bytes()))
			}

			sort.Strings(out)

			bytes := make([]byte, 0, len(out)*AddressLength)
			for _, k := range out {
				bytes = append(bytes, []byte(k)...)
			}
			return bytes, nil
		})).
	TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
		func(vals []byte) (AddrSet, error) {
			out := make(AddrSet)
			for i := 0; i < len(vals); i += AddressLength {
				end := i + AddressLength
				if end > len(vals) {
					end = len(vals)
				}
				s, err := NewAddressFromBytes(vals[i:end])
				if err != nil {
					return nil, err
				}
				out[s] = struct{}{}
			}
			return out, nil
		})).
	Complete()
