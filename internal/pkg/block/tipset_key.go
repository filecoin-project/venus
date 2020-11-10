package block

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/internal/pkg/enccid"
	"github.com/filecoin-project/venus/internal/pkg/encoding"
)

// TipSetKey is an immutable set of CIDs forming a unique key for a TipSet.
// Equal keys will have equivalent iteration order. CIDs are maintained in
// the same order as the canonical iteration order of blocks in a tipset (which is by ticket).
// This convention is maintained by the caller.  The order of input cids to the constructor
// must be the same as this canonical order.  It is the caller's responsibility to not
// construct a key with duplicate ids
// TipSetKey is a lightweight value type; passing by pointer is usually unnecessary.
type TipSetKey struct {
	// The slice is wrapped in a struct to enforce immutability.
	cids []enccid.Cid
}

// NewTipSetKey initialises a new TipSetKey.
// Duplicate CIDs are silently ignored.
func NewTipSetKey(ids ...cid.Cid) TipSetKey {
	if len(ids) == 0 {
		// Empty set is canonically represented by a nil slice rather than zero-length slice
		// so that a zero-value exactly matches an empty one.
		return TipSetKey{}
	}

	cids := make([]enccid.Cid, len(ids))
	for i := 0; i < len(ids); i++ {
		cids[i] = enccid.NewCid(ids[i])
	}
	return TipSetKey{cids}
}

// NewTipSetKeyFromUnique initialises a set with CIDs that are expected to be unique.
func NewTipSetKeyFromString(keyStr string) (TipSetKey, error) {
	keyStr = strings.Trim(keyStr, "{}")
	cidsStr := strings.Split(keyStr, ",")
	cids := make([]enccid.Cid, len(cidsStr))
	for i, cidStr := range cidsStr {
		id, err := cid.Decode(cidStr)
		if err != nil {
			return UndefTipSet.key, err
		}
		cids[i] = enccid.NewCid(id)
	}
	return TipSetKey{cids}, nil
}

// NewTipSetKeyFromUnique initialises a set with CIDs that are expected to be unique.
func NewTipSetKeyFromUnique(ids ...cid.Cid) (TipSetKey, error) {
	s := NewTipSetKey(ids...)
	if s.Len() != len(AsSet(ids)) {
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
		return !cidLess(s.cids[i].Cid, id)
	})
	return idx < len(s.cids) && s.cids[idx].Equals(id)
}

// Len returns the number of items in the set.
func (s TipSetKey) Len() int {
	return len(s.cids)
}

// ToSlice returns a slice listing the cids in the set.
func (s TipSetKey) ToSlice() []cid.Cid {
	return unwrap(s.cids)
}

// Iter returns an iterator that allows the caller to iterate the set in its sort order.
func (s TipSetKey) Iter() TipSetKeyIterator {
	return TipSetKeyIterator{
		s: s.ToSlice(),
		i: 0,
	}
}

// Equals checks whether the set contains exactly the same CIDs as another.
func (s TipSetKey) Equals(other TipSetKey) bool {
	if len(s.cids) != len(other.cids) {
		return false
	}
	for i := 0; i < len(s.cids); i++ {
		if !s.cids[i].Equals(other.cids[i].Cid) {
			return false
		}
	}
	return true
}

// ContainsAll checks if another set is a subset of this one.
// We can assume that the relative order of members of one key is
// maintained in the other since we assume that all ids are sorted
// by corresponding block ticket value.
func (s *TipSetKey) ContainsAll(other TipSetKey) bool {
	// Since we assume the ids must have the same relative sorting we can
	// perform one pass over this set, advancing the other index whenever the
	// values match.
	otherIdx := 0
	for i := 0; i < s.Len() && otherIdx < other.Len(); i++ {
		if s.cids[i].Equals(other.cids[otherIdx].Cid) {
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

// MarshalCBOR marshals the tipset key as an array of cids
func (s TipSetKey) MarshalCBOR() ([]byte, error) {
	// encode the zero value as length zero slice instead of nil per spec
	if s.cids == nil {
		encodableZero := make([]cid.Cid, 0)
		return encoding.Encode(encodableZero)
	}
	return encoding.Encode(s.cids)
}

// UnmarshalCBOR unmarshals a cbor array of cids to a tipset key
func (s *TipSetKey) UnmarshalCBOR(data []byte) error {
	var sortedEncCids []enccid.Cid
	err := encoding.Decode(data, &sortedEncCids)
	if err != nil {
		return err
	}
	sortedCids := unwrap(sortedEncCids)
	tmp, err := NewTipSetKeyFromUnique(sortedCids...)
	if err != nil {
		return err
	}
	*s = tmp
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

func cidLess(c1, c2 cid.Cid) bool {
	return c1.KeyString() < c2.KeyString()
}

// unwrap goes from a slice of encodable cids to a slice of cids
func unwrap(eCids []enccid.Cid) []cid.Cid {
	out := make([]cid.Cid, len(eCids))
	for i := 0; i < len(eCids); i++ {
		out[i] = eCids[i].Cid
	}
	return out
}

func AsSet(cids []cid.Cid) map[cid.Cid]struct{} {
	set := make(map[cid.Cid]struct{})
	for _, c := range cids {
		set[c] = struct{}{}
	}
	return set
}
