package block

import (
	"bytes"
	"sort"

	"github.com/filecoin-project/go-state-types/abi"
	fbig "github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"
)

// TipSet is a non-empty, immutable set of blocks at the same height with the same parent set.
// Blocks in a tipset are canonically ordered by ticket. Blocks may be iterated either via
// ToSlice() (which involves a shallow copy) or efficiently by index with At().
// TipSet is a lightweight value type; passing by pointer is usually unnecessary.
//
// Canonical tipset block ordering does not match the order of CIDs in a TipSetKey used as
// a tipset "key".
type TipSet struct {
	// This slice is wrapped in a struct to enforce immutability.
	blocks []*Block
	// Key is computed at construction and cached.
	key TipSetKey
}

var (
	// errNoBlocks is returned from the tipset constructor when given no blocks.
	errNoBlocks = errors.New("no blocks for tipset")
	// errUndefTipSet is returned from tipset methods invoked on an undefined tipset.
	errUndefTipSet = errors.New("undefined tipset")
)

// UndefTipSet is a singleton representing a nil or undefined tipset.
var UndefTipSet = &TipSet{}

// NewTipSet builds a new TipSet from a collection of blocks.
// The blocks must be distinct (different CIDs), have the same height, and same parent set.
func NewTipSet(blocks ...*Block) (*TipSet, error) {
	if len(blocks) == 0 {
		return nil, errNoBlocks
	}

	first := blocks[0]
	height := first.Height
	parents := first.Parents
	weight := first.ParentWeight
	cids := make([]cid.Cid, len(blocks))

	sorted := make([]*Block, len(blocks))
	for i, blk := range blocks {
		if i > 0 { // Skip redundant checks for first block
			if blk.Height != height {
				return nil, errors.Errorf("Inconsistent block heights %d and %d", height, blk.Height)
			}
			if !blk.Parents.Equals(parents) {
				return nil, errors.Errorf("Inconsistent block parents %s and %s", parents.String(), blk.Parents.String())
			}
			if !blk.ParentWeight.Equals(weight) {
				return nil, errors.Errorf("Inconsistent block parent weights %d and %d", weight, blk.ParentWeight)
			}
		}
		sorted[i] = blk
	}

	// Sort blocks by ticket
	sort.Slice(sorted, func(i, j int) bool {
		cmp := sorted[i].Ticket.Compare(&sorted[j].Ticket)
		if cmp == 0 {
			// Break ticket ties with the block CIDs, which are distinct.
			cmp = bytes.Compare(sorted[i].Cid().Bytes(), sorted[j].Cid().Bytes())
		}
		return cmp < 0
	})
	for i, blk := range sorted {
		cids[i] = blk.Cid()
	}
	// Duplicate blocks (CIDs) are rejected here, pass that error through.
	key, err := NewTipSetKeyFromUnique(cids...)
	if err != nil {
		return nil, err
	}
	return &TipSet{sorted, key}, nil
}

// Defined checks whether the tipset is defined.
// Invoking any other methods on an undefined tipset will result in undefined behaviour (c.f. cid.Undef)
func (ts *TipSet) Defined() bool {
	if ts == nil {
		return false
	}
	return len(ts.blocks) > 0
}

// Len returns the number of blocks in the tipset.
func (ts *TipSet) Len() int {
	if ts == nil {
		return 0
	}
	return len(ts.blocks)
}

// At returns the i'th block in the tipset.
// An index outside the half-open range [0, Len()) will panic.
func (ts *TipSet) At(i int) *Block {
	return ts.blocks[i]
}

// Key returns a key for the tipset.
func (ts *TipSet) Key() TipSetKey {
	return ts.key
}

// ToSlice returns an ordered slice of pointers to the tipset's blocks.
func (ts *TipSet) ToSlice() []*Block {
	slice := make([]*Block, len(ts.blocks))
	copy(slice, ts.blocks)
	return slice
}

// MinTicket returns the smallest ticket of all blocks in the tipset.
func (ts *TipSet) MinTicket() (Ticket, error) {
	if len(ts.blocks) == 0 {
		return Ticket{}, errUndefTipSet
	}
	return ts.blocks[0].Ticket, nil
}

// Height returns the height of a tipset.
func (ts *TipSet) Height() (abi.ChainEpoch, error) {
	if len(ts.blocks) == 0 {
		return 0, errUndefTipSet
	}
	return ts.blocks[0].Height, nil
}

// Height returns the height of a tipset.
func (ts *TipSet) EnsureHeight() abi.ChainEpoch {
	return ts.blocks[0].Height
}

// Parents returns the CIDs of the parents of the blocks in the tipset.
func (ts *TipSet) Parents() (TipSetKey, error) {
	if len(ts.blocks) == 0 {
		return TipSetKey{}, errUndefTipSet
	}
	return ts.blocks[0].Parents, nil
}

func (ts *TipSet) EnsureParents() TipSetKey {
	return ts.blocks[0].Parents
}

// ParentWeight returns the tipset's ParentWeight in fixed point form.
func (ts *TipSet) ParentWeight() (fbig.Int, error) {
	if len(ts.blocks) == 0 {
		return fbig.Zero(), errUndefTipSet
	}
	return ts.blocks[0].ParentWeight, nil
}

// Equals tests whether the tipset contains the same blocks as another.
// Equality is not tested deeply: two tipsets are considered equal if their keys (ordered block CIDs) are equal.
func (ts *TipSet) Equals(ts2 *TipSet) bool {
	if ts == nil && ts2 == nil {
		return true
	}
	if ts == nil || ts2 == nil {
		return false
	}

	return ts.Key().Equals(ts2.Key())
}

// String returns a formatted string of the CIDs in the TipSet.
// "{ <cid1> <cid2> <cid3> }"
// Note: existing callers use this as a unique key for the tipset. We should change them
// to use the TipSetKey explicitly
func (ts TipSet) String() string {
	return ts.Key().String()
}

func (ts *TipSet) Blocks() []*Block {
	return ts.blocks
}

func (ts *TipSet) MinTimestamp() uint64 {
	minTs := ts.blocks[0].Timestamp
	for _, bh := range ts.Blocks()[1:] {
		if bh.Timestamp < minTs {
			minTs = bh.Timestamp
		}
	}
	return minTs
}

func (ts *TipSet) IsChildOf(parent *TipSet) bool {
	return CidArrsEqual(ts.EnsureParents().ToSlice(), parent.key.ToSlice()) &&
		// FIXME: The height check might go beyond what is meant by
		//  "parent", but many parts of the code rely on the tipset's
		//  height for their processing logic at the moment to obviate it.
		ts.EnsureHeight() > parent.EnsureHeight()
}

func CidArrsEqual(a, b []cid.Cid) bool {
	if len(a) != len(b) {
		return false
	}

	// order ignoring compare...
	s := make(map[cid.Cid]bool)
	for _, c := range a {
		s[c] = true
	}

	for _, c := range b {
		if !s[c] {
			return false
		}
	}
	return true
}

func CidArrsSubset(a, b []cid.Cid) bool {
	// order ignoring compare...
	s := make(map[cid.Cid]bool)
	for _, c := range b {
		s[c] = true
	}

	for _, c := range a {
		if !s[c] {
			return false
		}
	}
	return true
}

func CidArrsContains(a []cid.Cid, b cid.Cid) bool {
	for _, elem := range a {
		if elem.Equals(b) {
			return true
		}
	}
	return false
}
