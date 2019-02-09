package consensus

import (
	"bytes"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/types"
)

// Tip is what expected consensus needs from a Block. For now it *is* a
// Block.
type Tip = types.Block

// TipSet is a set of Tips, blocks at the same height with the same parent set,
// keyed by Cid string.
type TipSet map[string]*Tip

var (
	// ErrEmptyTipSet is returned when a method requiring a non-empty tipset is called on an empty tipset
	ErrEmptyTipSet = errors.New("empty tipset calling unallowed method")
)

// NewTipSet returns a TipSet wrapping the input blocks.
// PRECONDITION: all blocks are the same height and have the same parent set.
func NewTipSet(blks ...*types.Block) (TipSet, error) {
	if len(blks) == 0 {
		return nil, errors.New("Cannot create tipset: no blocks")
	}
	ts := TipSet{}
	for _, b := range blks {
		if err := ts.AddBlock(b); err != nil {
			return nil, errors.Wrapf(err, "Cannot create tipset")
		}
	}
	return ts, nil
}

// AddBlock adds the provided block to this tipset.
// PRECONDITION: this block has the same height parent set as other members of ts.
func (ts TipSet) AddBlock(b *types.Block) error {
	if len(ts) == 0 {
		id := b.Cid()
		ts[id.String()] = b
		return nil
	}

	h, err := ts.Height()
	if err != nil {
		return err
	}
	p, err := ts.Parents()
	if err != nil {
		return err
	}
	weight, err := ts.ParentWeight()
	if err != nil {
		return err
	}
	if uint64(b.Height) != h {
		return errors.Errorf("block height %d doesn't match existing tipset height %d", uint64(b.Height), h)
	}
	if !b.Parents.Equals(p) {
		return errors.Errorf("block parents %s don't match tipset parents %s", b.Parents.String(), p.String())
	}
	if uint64(b.ParentWeight) != weight {
		return errors.Errorf("bBlock parent weight: %d doesn't match existing tipset parent weight: %d", uint64(b.ParentWeight), weight)
	}

	id := b.Cid()
	ts[id.String()] = b
	return nil
}

// Clone returns a shallow copy of the TipSet.
func (ts TipSet) Clone() TipSet {
	r := TipSet{}
	for k, v := range ts {
		r[k] = v
	}

	return r
}

// String returns a formatted string of the TipSet:
// { <cid1> <cid2> <cid3> }
func (ts TipSet) String() string {
	return ts.ToSortedCidSet().String()
}

// Equals returns true if the tipset contains the same blocks as another set.
// Equality is not tested deeply.  If blocks of two tipsets are stored at
// different memory addresses but have the same cids the tipsets will be equal.
func (ts TipSet) Equals(ts2 TipSet) bool {
	return ts.ToSortedCidSet().Equals(ts2.ToSortedCidSet())
}

// ToSortedCidSet returns a SortedCidSet containing the Cids in the
// TipSet.
func (ts TipSet) ToSortedCidSet() types.SortedCidSet {
	s := types.SortedCidSet{}
	for _, b := range ts {
		s.Add(b.Cid())
	}
	return s
}

// ToSlice returns the slice of *Block containing the tipset's blocks.
// Sorted.
func (ts TipSet) ToSlice() []*types.Block {
	sl := make([]*types.Block, len(ts))
	var i int
	for it := ts.ToSortedCidSet().Iter(); !it.Complete(); it.Next() {
		sl[i] = ts[it.Value().String()]
		i++
	}
	return sl
}

// MinTicket returns the smallest ticket of all blocks in the tipset.
func (ts TipSet) MinTicket() (types.Signature, error) {
	if len(ts) == 0 {
		return nil, ErrEmptyTipSet
	}
	blks := ts.ToSlice()
	min := blks[0].Ticket
	for i := range blks[0:] {
		if bytes.Compare(blks[i].Ticket, min) < 0 {
			min = blks[i].Ticket
		}
	}
	return min, nil
}

// Height returns the height of a tipset.
func (ts TipSet) Height() (uint64, error) {
	if len(ts) == 0 {
		return uint64(0), ErrEmptyTipSet
	}
	return uint64(ts.ToSlice()[0].Height), nil
}

// Parents returns the parents of a tipset.
func (ts TipSet) Parents() (types.SortedCidSet, error) {
	if len(ts) == 0 {
		return types.SortedCidSet{}, ErrEmptyTipSet
	}
	return ts.ToSlice()[0].Parents, nil
}

// ParentWeight returns the tipset's ParentWeight in fixed point form.
func (ts TipSet) ParentWeight() (uint64, error) {
	if len(ts) == 0 {
		return uint64(0), ErrEmptyTipSet
	}
	return uint64(ts.ToSlice()[0].ParentWeight), nil
}
