package types

import (
	"encoding/json"
	"fmt"
	"sort"

	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"
	"gx/ipfs/QmdBzoMxsBpojBfN1cv5GnKtB7sfYBMoLH7p9qSyEVYXcu/refmt/obj/atlas"
)

func init() {
	cbor.RegisterCborType(atlas.BuildEntry(SortedCidSet{}).Transform().
		TransformMarshal(atlas.MakeMarshalTransformFunc(
			func(s SortedCidSet) ([]cid.Cid, error) {
				return s.s, nil
			})).
		TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
			func(s []cid.Cid) (SortedCidSet, error) {
				for i := 0; i < len(s)-1; i++ {
					// Note that this will also catch duplicates.
					if !cidLess(s[i], s[i+1]) {
						return SortedCidSet{}, fmt.Errorf(
							"invalid serialization of SortedCidSet - %s not less than %s", s[i].String(), s[i+1].String())
					}
				}
				return SortedCidSet{s: s}, nil
			})).
		Complete())
}

// SortedCidSet is a set of Cids that is maintained sorted. The externally visible effect as
// compared to cid.Set is that iteration is cheap and always in-order.
// Sort order is lexicographic ascending, by serialization of the cid.
// TODO: This should probably go into go-cid package - see https://github.com/ipfs/go-cid/issues/45.
type SortedCidSet struct {
	s []cid.Cid // should be maintained sorted
}

// NewSortedCidSet returns a SortedCidSet with the specified items.
func NewSortedCidSet(ids ...cid.Cid) (res SortedCidSet) {
	for _, id := range ids {
		res.Add(id)
	}
	return
}

// Add adds a cid to the set. Returns true if the item was added (didn't already exist), false
// otherwise.
func (s *SortedCidSet) Add(id cid.Cid) bool {
	idx := s.search(id)
	if idx < len(s.s) && s.s[idx].Equals(id) {
		return false
	}
	s.s = append(s.s, cid.Undef)
	copy(s.s[idx+1:], s.s[idx:])
	s.s[idx] = id
	return true
}

// Has returns true if the set contains the specified cid.
func (s SortedCidSet) Has(id cid.Cid) bool {
	idx := s.search(id)
	return idx < len(s.s) && s.s[idx].Equals(id)
}

// Len returns the number of items in the set.
func (s SortedCidSet) Len() int {
	return len(s.s)
}

// Empty returns true if the set is empty.
func (s SortedCidSet) Empty() bool {
	return s.Len() == 0
}

// Remove removes a cid from the set. Returns true if the item was removed (did in fact exist in
// the set), false otherwise.
func (s *SortedCidSet) Remove(id cid.Cid) bool {
	idx := s.search(id)
	if idx < len(s.s) && s.s[idx].Equals(id) {
		copy(s.s[idx:], s.s[idx+1:])
		s.s = s.s[0 : len(s.s)-1]
		return true
	}
	return false
}

// Clear removes all entries from the set.
func (s *SortedCidSet) Clear() {
	s.s = s.s[:0]
}

// Iter returns an iterator that allows the caller to iterate the set in its sort order.
func (s SortedCidSet) Iter() SortedCidSetIterator {
	return SortedCidSetIterator{
		s: s.s,
		i: 0,
	}
}

// Equals returns true if the set contains the same items as another set.
func (s SortedCidSet) Equals(s2 SortedCidSet) bool {
	if s.Len() != s2.Len() {
		return false
	}

	i1 := s.Iter()
	i2 := s2.Iter()

	for i := 0; i < s.Len(); i++ {
		if !i1.Value().Equals(i2.Value()) {
			return false
		}
	}

	return true
}

// Contains checks if s2 is a sub-tipset of s
func (s *SortedCidSet) Contains(s2 *SortedCidSet) bool {
	for it := s2.Iter(); !it.Complete(); it.Next() {
		if !s.Has(it.Value()) {
			return false
		}
	}
	return true
}

// String returns a string listing the cids in the set.
func (s SortedCidSet) String() string {
	out := "{"
	for it := s.Iter(); !it.Complete(); it.Next() {
		out = fmt.Sprintf("%s %s", out, it.Value().String())
	}
	return out + " }"
}

// ToSlice returns a slice listing the cids in the set.
func (s SortedCidSet) ToSlice() []cid.Cid {
	out := make([]cid.Cid, s.Len())
	var i int
	for it := s.Iter(); !it.Complete(); it.Next() {
		out[i] = it.Value()
		i++
	}
	return out
}

// MarshalJSON serializes the set to JSON.
func (s SortedCidSet) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.s)
}

// UnmarshalJSON parses JSON into the set.
func (s *SortedCidSet) UnmarshalJSON(b []byte) error {
	var ts []cid.Cid
	if err := json.Unmarshal(b, &ts); err != nil {
		return err
	}
	for i := 0; i < len(ts)-1; i++ {
		if !cidLess(ts[i], ts[i+1]) {
			return fmt.Errorf("invalid input - cids not sorted")
		}
	}
	s.s = ts
	return nil
}

func (s SortedCidSet) search(id cid.Cid) int {
	return sort.Search(len(s.s), func(i int) bool {
		return !cidLess(s.s[i], id)
	})
}

// SortedCidSetIterator is a iterator over a sorted collection of CIDs.
type SortedCidSetIterator struct {
	s []cid.Cid
	i int
}

// Complete returns true if the iterator has reached the end of the set.
func (si *SortedCidSetIterator) Complete() bool {
	return si.i >= len(si.s)
}

// Next advances the iterator to the next item and returns true if there is such an item.
func (si *SortedCidSetIterator) Next() bool {
	switch {
	case si.i < len(si.s):
		si.i++
		return si.i < len(si.s)
	case si.i == len(si.s):
		return false
	default:
		panic("unreached")
	}
}

// Value returns the current item for the iterator
func (si SortedCidSetIterator) Value() cid.Cid {
	switch {
	case si.i < len(si.s):
		return si.s[si.i]
	case si.i == len(si.s):
		return cid.Undef
	default:
		panic("unreached")
	}
}

func cidLess(c1, c2 cid.Cid) bool {
	return c1.KeyString() < c2.KeyString()
}
