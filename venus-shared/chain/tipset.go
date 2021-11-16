package chain

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
)

type blockHeaderWithCid struct {
	c cid.Cid
	b *BlockHeader
}

func NewTipSet(bhs []*BlockHeader) (*TipSet, error) {
	if len(bhs) == 0 {
		return nil, fmt.Errorf("no blocks for tipset")
	}

	blks := make([]*blockHeaderWithCid, len(bhs))
	first := bhs[0]
	blks[0] = &blockHeaderWithCid{
		c: first.Cid(),
		b: first,
	}

	seen := make(map[cid.Cid]struct{})
	seen[blks[0].c] = struct{}{}

	for i := 1; i < len(bhs); i++ {
		blk := bhs[i]
		if blk.Height != first.Height {
			return nil, fmt.Errorf("inconsistent block heights %d and %d", first.Height, blk.Height)
		}

		if !sortedCidArrsEqual(blk.Parents, first.Parents) {
			return nil, fmt.Errorf("inconsistent block parents %s and %s", NewTipSetKey(first.Parents...), NewTipSetKey(blk.Parents...))
		}

		if !blk.ParentWeight.Equals(first.ParentWeight) {
			return nil, fmt.Errorf("inconsistent block parent weights %d and %d", first.ParentWeight, blk.ParentWeight)
		}

		bcid := blk.Cid()
		if _, ok := seen[bcid]; ok {
			return nil, fmt.Errorf("duplicate block %s", bcid)
		}

		seen[bcid] = struct{}{}
		blks[i] = &blockHeaderWithCid{
			c: bcid,
			b: blk,
		}
	}

	sortBlockHeadersInTipSet(blks)
	blocks := make([]*BlockHeader, len(blks))
	cids := make([]cid.Cid, len(blks))
	for i := range blks {
		blocks[i] = blks[i].b
		cids[i] = blks[i].c
	}

	return &TipSet{
		blocks: blocks,

		key:  NewTipSetKey(cids...),
		cids: cids,

		height: first.Height,

		parentsKey: NewTipSetKey(first.Parents...),
	}, nil
}

// TipSet is a non-empty, immutable set of blocks at the same height with the same parent set.
// Blocks in a tipset are canonically ordered by ticket. Blocks may be iterated either via
// ToSlice() (which involves a shallow copy) or efficiently by index with At().
// TipSet is a lightweight value type; passing by pointer is usually unnecessary.
//
// Canonical tipset newBlock ordering does not match the order of CIDs in a TipSetKey used as
// a tipset "key".
type TipSet struct {
	// This slice is wrapped in a struct to enforce immutability.
	blocks []*BlockHeader
	// Key is computed at construction and cached.
	key  TipSetKey
	cids []cid.Cid

	height abi.ChainEpoch

	parentsKey TipSetKey
}

// Defined checks whether the tipset is defined.
// Invoking any other methods on an undefined tipset will result in undefined behaviour (c.f. cid.Undef)
func (ts *TipSet) Defined() bool {
	return ts != nil && len(ts.blocks) > 0
}

func (ts *TipSet) Equals(ots *TipSet) bool {
	if ts == nil && ots == nil {
		return true
	}
	if ts == nil || ots == nil {
		return false
	}

	if ts.height != ots.height {
		return false
	}

	if len(ts.cids) != len(ots.cids) {
		return false
	}

	for i, cid := range ts.cids {
		if cid != ots.cids[i] {
			return false
		}
	}

	return true
}

// Len returns the number of blocks in the tipset.
func (ts *TipSet) Len() int {
	if ts == nil {
		return 0
	}
	return len(ts.blocks)
}

func (ts *TipSet) Blocks() []*BlockHeader {
	return ts.blocks
}

// Key returns a key for the tipset.
func (ts *TipSet) Key() TipSetKey {
	if ts == nil {
		return EmptyTSK
	}
	return ts.key
}

func (ts *TipSet) Cids() []cid.Cid {
	if !ts.Defined() {
		return []cid.Cid{}
	}

	dst := make([]cid.Cid, len(ts.cids))
	copy(dst, ts.cids)
	return dst
}

// Height returns the height of a tipset.
func (ts *TipSet) Height() abi.ChainEpoch {
	if ts.Defined() {
		return ts.height
	}

	return 0
}

// Parents returns the CIDs of the parents of the blocks in the tipset.
func (ts *TipSet) Parents() TipSetKey {
	if ts.Defined() {
		return ts.parentsKey
	}

	return EmptyTSK
}

// Parents returns the CIDs of the parents of the blocks in the tipset.
func (ts *TipSet) ParentState() cid.Cid {
	if ts.Defined() {
		return ts.blocks[0].ParentStateRoot
	}
	return cid.Undef
}

// ParentWeight returns the tipset's ParentWeight in fixed point form.
func (ts *TipSet) ParentWeight() big.Int {
	if ts.Defined() {
		return ts.blocks[0].ParentWeight
	}
	return big.Zero()
}

// String returns a formatted string of the CIDs in the TipSet.
// "{ <cid1> <cid2> <cid3> }"
// Note: existing callers use this as a unique key for the tipset. We should change them
// to use the TipSetKey explicitly
func (ts TipSet) String() string {
	return ts.Key().String()
}

func (ts *TipSet) IsChildOf(parent *TipSet) bool {
	return cidArrsEqual(ts.Parents().Cids(), parent.key.Cids()) &&
		// FIXME: The height check might go beyond what is meant by
		//  "parent", but many parts of the code rely on the tipset's
		//  height for their processing logic at the moment to obviate it.
		ts.Height() > parent.Height()
}

func (ts *TipSet) MinTicketBlock() *BlockHeader {
	min := ts.blocks[0]

	for _, b := range ts.blocks[1:] {
		if b.LastTicket().Less(min.LastTicket()) {
			min = b
		}
	}

	return min
}

// MinTicket returns the smallest ticket of all blocks in the tipset.
func (ts *TipSet) MinTicket() Ticket {
	return ts.MinTicketBlock().Ticket
}

func (ts *TipSet) MinTimestamp() uint64 {
	minTS := ts.blocks[0].Timestamp
	for _, bh := range ts.blocks[1:] {
		if bh.Timestamp < minTS {
			minTS = bh.Timestamp
		}
	}
	return minTS
}

func sortedCidArrsEqual(a, b []cid.Cid) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func sortBlockHeadersInTipSet(blks []*blockHeaderWithCid) {
	sort.Slice(blks, func(i, j int) bool {
		cmp := blks[i].b.Ticket.Compare(&blks[j].b.Ticket)
		if cmp == 0 {
			// Break ticket ties with the newBlock CIDs, which are distinct.
			cmp = bytes.Compare(blks[i].c.Bytes(), blks[j].c.Bytes())
		}
		return cmp < 0
	})
}

func cidArrsEqual(a, b []cid.Cid) bool {
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
