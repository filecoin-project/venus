package core

import (
	"bytes"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/types"
)

// tipIndex tracks tipsets by height and parent set, mainly for use in expected consensus.
type tipIndex map[uint64]tipSetsByParents

func (ti tipIndex) addBlock(b *types.Block) error {
	tsbp, ok := ti[uint64(b.Height)]
	if !ok {
		tsbp = tipSetsByParents{}
		ti[uint64(b.Height)] = tsbp
	}
	return tsbp.addBlock(b)
}

type tipSetsByParents map[string]TipSet

func (tsbp tipSetsByParents) addBlock(b *types.Block) error {
	key := keyForParentSet(b.Parents)
	ts := tsbp[key]
	if ts == nil {
		ts = TipSet{}
	}
	err := ts.AddBlock(b)
	if err != nil {
		return err
	}
	tsbp[key] = ts
	return nil
}

func keyForParentSet(parents types.SortedCidSet) string {
	var k string
	for it := parents.Iter(); !it.Complete(); it.Next() {
		k += it.Value().String()
	}
	return k
}

// Tip is what expected consensus needs from a Block. For now it *is* a
// Block.
// TODO This needs to change in the future as holding all Blocks in
// memory is expensive. We could define a struct encompassing the subset
// of Block needed for EC and embed it in the block or we could limit the
// height we index or both.
type Tip = types.Block

// TipSet is a set of Tips, blocks at the same height with the same parent set,
// keyed by Cid string.
type TipSet map[string]*Tip

var (
	// ErrBadTipSetCreate is returned when there is an error creating a new tipset
	ErrBadTipSetCreate = errors.New("tipset contains blocks of different heights, or different parent sets or weights")
	// ErrBadTipSetAdd is returned when there is an error adding a block to a tipset
	ErrBadTipSetAdd = errors.New("block has invalid height, parent set or parent weight to be a member of tipset")
	// ErrEmptyTipSet is returned when a method requiring a non-empty tipset is called on an empty tipset
	ErrEmptyTipSet = errors.New("empty tipset calling unallowed method")
)

// NewTipSet returns a TipSet wrapping the input blocks.
// PRECONDITION: all blocks are the same height and have the same parent set.
func NewTipSet(blks ...*types.Block) (TipSet, error) {
	if len(blks) == 0 {
		return nil, ErrBadTipSetCreate
	}
	ts := TipSet{}
	for _, b := range blks {
		if err := ts.AddBlock(b); err != nil {
			return nil, ErrBadTipSetCreate
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
	pW, err := ts.ParentWeight()
	if err != nil {
		return err
	}
	if uint64(b.Height) != h || !b.Parents.Equals(p) || uint64(b.ParentWeight) != pW {
		return ErrBadTipSetAdd
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

// ToSortedCidSet returns a types.SortedCidSet containing the Cids in the
// TipSet.
func (ts TipSet) ToSortedCidSet() types.SortedCidSet {
	s := types.SortedCidSet{}
	for _, b := range ts {
		s.Add(b.Cid())
	}
	return s
}

// ToSlice returns the slice of *types.Block containing the tipset's blocks.
func (ts TipSet) ToSlice() []*types.Block {
	sl := make([]*types.Block, len(ts))
	var i int
	for _, b := range ts {
		sl[i] = b
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
	for i := range blks[1:] {
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
	if len(ts) < 0 {
		return types.SortedCidSet{}, ErrEmptyTipSet
	}
	return ts.ToSlice()[0].Parents, nil
}

// ParentWeight returns the ParentWeight of the TipSet.
func (ts TipSet) ParentWeight() (uint64, error) {
	if len(ts) == 0 {
		return uint64(0), ErrEmptyTipSet
	}
	return uint64(ts.ToSlice()[0].ParentWeight), nil
}

// BaseBlockFromTipSets is a likely TEMPORARY helper to extract a base block
// from a tipset. Prior to EC the mining worker mined off of a base block. With
// EC it is mining off of a set of TipSets. We haven't plumbed the change from
// block to TipSets all the way through yet, hence this function which extracts
// a base block from the TipSets.
func BaseBlockFromTipSets(tipSets []TipSet) *types.Block {
	tipSet := tipSets[0]
	var tipSetKey string
	for k := range tipSet {
		tipSetKey = k
		break
	}
	return tipSet[tipSetKey]
}
