package core

import (
	"github.com/filecoin-project/go-filecoin/types"
)

// tipIndex tracks tipsets by height and parent set, mainly for use in expected consensus.
type tipIndex map[uint64]tipSetsByParents

func (ti tipIndex) addBlock(b *types.Block) {
	tsbp, ok := ti[b.Height]
	if !ok {
		tsbp = tipSetsByParents{}
		ti[b.Height] = tsbp
	}
	tsbp.addBlock(b)
}

type tipSetsByParents map[string]TipSet

func (tsbp tipSetsByParents) addBlock(b *types.Block) {
	key := keyForParentSet(b.Parents)
	ts := tsbp[key]
	if ts == nil {
		ts = TipSet{}
	}
	id := b.Cid()
	ts[id.String()] = b
	tsbp[key] = ts
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

// Clone returns a shallow copy of the TipSet.
func (ts TipSet) Clone() TipSet {
	r := TipSet{}
	for k, v := range ts {
		r[k] = v
	}
	return r
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
