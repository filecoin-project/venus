package storagemarket

import (
	"sort"

	cbor "gx/ipfs/QmV6BQ6fFCf9eFHDuRxvguvqfKLZtZrxthgZvDfRCs4tMN/go-ipld-cbor"
	"gx/ipfs/QmcrriCMhjb5ZWzmPNxmP53px47tSPcXBNaMtLdgcKFJYk/refmt/obj/atlas"
)

func init() {
	cbor.RegisterCborType(askSetEntry)
	cbor.RegisterCborType(Ask{})
	cbor.RegisterCborType(bidSetEntry)
	cbor.RegisterCborType(Bid{})
}

// AskSet is a convenience type for sets of Asks
// For now, its just a simple map. In the future, there will be too many asks
// to load all into memory at once and we will have to do something more clever
// here.
type AskSet map[uint64]*Ask

// refmt doesnt like maps with integers as keys, mostly because the semantics
// are not defined clearly between different serialization formats. Its
// something we could get implemented, but it adds a lot of code and a
// performance hit (cbor specifies that all map keys are sorted as strings, so
// we would have to convert every single int into a string, then sort by that)
// Plus, storing them as an array is more space efficient.
// TODO: figure out how interacting with large amounts of storage is going to
// work.
var askSetEntry = atlas.BuildEntry(AskSet{}).Transform().
	TransformMarshal(atlas.MakeMarshalTransformFunc(
		func(s AskSet) ([]*Ask, error) {
			// TODO: theres probably a more efficient way of doing this. But this works for now.
			out := make([]*Ask, 0, len(s))
			for _, ask := range s {
				out = append(out, ask)
			}

			sort.Slice(out, func(i, j int) bool {
				return out[i].ID < out[j].ID
			})

			return out, nil
		})).
	TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
		func(x []*Ask) (AskSet, error) {
			out := make(AskSet)
			for _, v := range x {
				out[v.ID] = v
			}
			return out, nil
		})).
	Complete()

// BidSet is a convenience type for sets of Bids
type BidSet map[uint64]*Bid

// refmt doesnt like maps with integers as keys, mostly because the semantics
// are not defined clearly between different serialization formats. Its
// something we could get implemented, but it adds a lot of code and a
// performance hit (cbor specifies that all map keys are sorted as strings, so
// we would have to convert every single int into a string, then sort by that)
// Plus, storing them as an array is more space efficient.
// TODO: figure out how interacting with large amounts of storage is going to
// work.
var bidSetEntry = atlas.BuildEntry(BidSet{}).Transform().
	TransformMarshal(atlas.MakeMarshalTransformFunc(
		func(s BidSet) ([]*Bid, error) {
			// TODO: theres probably a more efficient way of doing this. But this works for now.
			out := make([]*Bid, 0, len(s))
			for _, bid := range s {
				out = append(out, bid)
			}

			sort.Slice(out, func(i, j int) bool {
				return out[i].ID < out[j].ID
			})

			return out, nil
		})).
	TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
		func(x []*Bid) (BidSet, error) {
			out := make(BidSet)
			for _, v := range x {
				out[v.ID] = v
			}
			return out, nil
		})).
	Complete()
