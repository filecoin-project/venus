package block

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"
	"github.com/polydawn/refmt/obj/atlas"
)

func init() {
	// A TipSetKey serializes as a sorted array of CIDs.
	// Deserialization will sort the CIDs, if they're not already.
	encoding.RegisterIpldCborType(atlas.BuildEntry(TipSetKey{}).Transform().
		TransformMarshal(atlas.MakeMarshalTransformFunc(
			func(s TipSetKey) ([]cid.Cid, error) {
				return s.cids, nil
			})).
		TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
			func(cids []cid.Cid) (TipSetKey, error) {
				return NewTipSetKeyFromUnique(cids...)
			})).
		Complete())
}

// TipSetKey is an immutable set of CIDs forming a unique key for a TipSet.
// Equal keys will have equivalent iteration order, but note that the CIDs are *not* maintained in
// the same order as the canonical iteration order of blocks in a tipset (which is by ticket).
// TipSetKey is a lightweight value type; passing by pointer is usually unnecessary.
type TipSetKey struct {
	// The slice is wrapped in a struct to enforce immutability.
	cids []cid.Cid
}

// NewTipSetKey initialises a new TipSetKey.
// Duplicate CIDs are silently ignored.
func NewTipSetKey(ids ...cid.Cid) TipSetKey {
	if len(ids) == 0 {
		// Empty set is canonically represented by a nil slice rather than zero-length slice
		// so that a zero-value exactly matches an empty one.
		return TipSetKey{}
	}

	cids := make([]cid.Cid, len(ids))
	copy(cids, ids)
	return TipSetKey{uniq(cids)}
}

// NewTipSetKeyFromUnique initialises a set with CIDs that are expected to be unique.
func NewTipSetKeyFromUnique(ids ...cid.Cid) (TipSetKey, error) {
	s := NewTipSetKey(ids...)
	if s.Len() != len(ids) {
		return TipSetKey{}, errors.Errorf("Duplicate CID in %s", ids)
	}
	return s, nil
}

// Empty checks whether the set is empty.
func (s TipSetKey) Empty() bool {
	return s.Len() == 0
}

// Has checks whether the set contains `id`.
func (s TipSetKey) Has(id cid.Cid) bool {
	// Find index of the first CID not less than id.
	idx := sort.Search(len(s.cids), func(i int) bool {
		return !cidLess(s.cids[i], id)
	})
	return idx < len(s.cids) && s.cids[idx].Equals(id)
}

// Len returns the number of items in the set.
func (s TipSetKey) Len() int {
	return len(s.cids)
}

// ToSlice returns a slice listing the cids in the set.
func (s TipSetKey) ToSlice() []cid.Cid {
	out := make([]cid.Cid, len(s.cids))
	copy(out, s.cids)
	return out
}

// Iter returns an iterator that allows the caller to iterate the set in its sort order.
func (s TipSetKey) Iter() TipSetKeyIterator {
	return TipSetKeyIterator{
		s: s.cids,
		i: 0,
	}
}

// Equals checks whether the set contains exactly the same CIDs as another.
func (s TipSetKey) Equals(other TipSetKey) bool {
	if len(s.cids) != len(other.cids) {
		return false
	}
	for i := 0; i < len(s.cids); i++ {
		if !s.cids[i].Equals(other.cids[i]) {
			return false
		}
	}
	return true
}

// ContainsAll checks if another set is a subset of this one.
func (s *TipSetKey) ContainsAll(other TipSetKey) bool {
	// Since the slices are sorted we can perform one pass over this set, advancing
	// the other index whenever the values match.
	otherIdx := 0
	for i := 0; i < s.Len() && otherIdx < other.Len(); i++ {
		if s.cids[i].Equals(other.cids[otherIdx]) {
			otherIdx++
		}
	}
	// otherIdx is advanced the full length only if every element was found in this set.
	return otherIdx == other.Len()
}

// String returns a string listing the cids in the set.
func (s TipSetKey) String() string {
	out := "{"
	for it := s.Iter(); !it.Complete(); it.Next() {
		out = fmt.Sprintf("%s %s", out, it.Value().String())
	}
	return out + " }"
}

// MarshalJSON serializes the key to JSON.
func (s TipSetKey) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.cids)
}

// UnmarshalJSON parses JSON into the key.
// Note that this pattern technically violates the immutability.
func (s *TipSetKey) UnmarshalJSON(b []byte) error {
	var cids []cid.Cid
	if err := json.Unmarshal(b, &cids); err != nil {
		return err
	}

	k, err := NewTipSetKeyFromUnique(cids...)
	if err != nil {
		return err
	}
	s.cids = k.cids
	return nil
}

// TipSetKeyIterator is a iterator over a sorted collection of CIDs.
type TipSetKeyIterator struct {
	s []cid.Cid
	i int
}

// Complete returns true if the iterator has reached the end of the set.
func (si *TipSetKeyIterator) Complete() bool {
	return si.i >= len(si.s)
}

// Next advances the iterator to the next item and returns true if there is such an item.
func (si *TipSetKeyIterator) Next() bool {
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
func (si TipSetKeyIterator) Value() cid.Cid {
	switch {
	case si.i < len(si.s):
		return si.s[si.i]
	case si.i == len(si.s):
		return cid.Undef
	default:
		panic("unreached")
	}
}

// Destructively sorts and uniqifies a slice of CIDs.
func uniq(cids []cid.Cid) []cid.Cid {
	sort.Slice(cids, func(i, j int) bool {
		return cidLess(cids[i], cids[j])
	})

	if len(cids) >= 2 {
		// Uniq-ify the sorted array.
		// You can imagine doing this by using a second slice and appending elements to it if
		// they don't match the last-copied element. This is just the same, but using the
		// source slice as the destination too.
		this, next := 0, 1
		for next < len(cids) {
			if cids[next] != cids[this] {
				this++
				cids[this] = cids[next]
			}
			next++
		}
		return cids[:this+1]
	}
	return cids
}

func cidLess(c1, c2 cid.Cid) bool {
	return c1.KeyString() < c2.KeyString()
}
