package types

import (
	"bytes"
	"sort"

	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"
)

// TipSet is a non-empty, immutable set of blocks at the same height with the same parent set.
// Blocks in a tipset are canonically ordered by ticket. Blocks may be iterated either via
// ToSlice() (which involves a shallow copy) or efficiently by index with At().
// TipSet is a lightweight value type; passing by pointer is usually unnecessary.
//
// Canonical tipset block ordering does not match the order of CIDs in a SortedCidSet used as
// a tipset "key".
type TipSet struct {
	// This slice is wrapped in a struct to enforce immutability.
	blocks []*Block
}

var (
	// ErrEmptyTipSet is returned no blocks are provided to a tipset contructor
	ErrEmptyTipSet = errors.New("no blocks for tipset")
)

// UndefTipSet is a singleton representing a nil or undefined tipset.
var UndefTipSet = TipSet{}

// NewTipSet builds a new TipSet from a collection of blocks.
// The blocks must be distinct (different CIDs), have the same height, and same parent set.
func NewTipSet(blocks ...*Block) (TipSet, error) {
	if len(blocks) == 0 {
		return UndefTipSet, ErrEmptyTipSet
	}

	first := blocks[0]
	height := first.Height
	parents := first.Parents
	weight := first.ParentWeight
	cids := make(map[cid.Cid]bool)

	sorted := make([]*Block, len(blocks))
	for i, blk := range blocks {
		if i > 0 { // Skip redundant checks for first block
			if blk.Height != height {
				return UndefTipSet, errors.Errorf("Inconsistent block heights %d and %d", height, blk.Height)
			}
			if !blk.Parents.Equals(parents) {
				return UndefTipSet, errors.Errorf("Inconsistent block parents %s and %s", parents.String(), blk.Parents.String())
			}
			if blk.ParentWeight != weight {
				return UndefTipSet, errors.Errorf("Inconsistent block parent weights %d and %d", weight, blk.ParentWeight)
			}
		}
		// Reject duplicate blocks (by CID).
		c := blk.Cid()
		if cids[c] {
			return UndefTipSet, errors.Errorf("Duplicate block CID %s", c)
		}
		cids[c] = true
		sorted[i] = blk
	}

	// Sort blocks by ticket.
	sort.Slice(sorted, func(i, j int) bool {
		cmp := bytes.Compare(sorted[i].Ticket, sorted[j].Ticket)
		if cmp == 0 {
			// Break ticket ties with the block CIDs, which are distinct.
			cmp = bytes.Compare(sorted[i].Cid().Bytes(), sorted[j].Cid().Bytes())
		}
		return cmp < 0
	})

	return TipSet{sorted}, nil
}

// Defined checks whether the tipset is defined.
// Invoking any other methods on an undefined tipset will result in undefined behaviour (c.f. cid.Undef)
func (ts TipSet) Defined() bool {
	return len(ts.blocks) > 0
}

// Len returns the number of blocks in the tipset.
func (ts TipSet) Len() int {
	return len(ts.blocks)
}

// At returns the i'th block in the tipset.
// An index outside the half-open range [0, Len()) will panic.
func (ts TipSet) At(i int) *Block {
	return ts.blocks[i]
}

// ToSortedCidSet returns a SortedCidSet containing the CIDs in the tipset.
func (ts TipSet) ToSortedCidSet() SortedCidSet {
	s := SortedCidSet{}
	for _, b := range ts.blocks {
		s.Add(b.Cid())
	}
	return s
}

// ToSlice returns an ordered slice of pointers to the tipset's blocks.
func (ts TipSet) ToSlice() []*Block {
	slice := make([]*Block, len(ts.blocks))
	copy(slice, ts.blocks)
	return slice
}

// MinTicket returns the smallest ticket of all blocks in the tipset, and nil error.
// The nil error is to be removed shortly.
func (ts TipSet) MinTicket() (Signature, error) {
	return ts.blocks[0].Ticket, nil
}

// Height returns the height of a tipset, and nil error.
func (ts TipSet) Height() (uint64, error) {
	return uint64(ts.blocks[0].Height), nil
}

// Parents returns the CIDs of the parents of the blocks in the tipset, and nil error.
func (ts TipSet) Parents() (SortedCidSet, error) {
	return ts.blocks[0].Parents, nil
}

// ParentWeight returns the tipset's ParentWeight in fixed point form, and nil error.
func (ts TipSet) ParentWeight() (uint64, error) {
	return uint64(ts.blocks[0].ParentWeight), nil
}

// Equals tests whether the tipset contains the same blocks as another.
// Equality is not tested deeply: two tipsets are considered equal if their keys (ordered block CIDs) are equal.
func (ts TipSet) Equals(ts2 TipSet) bool {
	return ts.ToSortedCidSet().Equals(ts2.ToSortedCidSet())
}

// String returns a formatted string of the CIDs in the TipSet.
// "{ <cid1> <cid2> <cid3> }"
// Note: existing callers use this as a unique key for the tipset. We should change them
// to use the sorted CID set explicitly
func (ts TipSet) String() string {
	return ts.ToSortedCidSet().String()
}
