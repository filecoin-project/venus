package core

import (
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

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

type tipSetsByParents map[string]tipSet

func (tsbp tipSetsByParents) addBlock(b *types.Block) {
	key := keyForParentSet(b.Parents())
	ts := tsbp[key]
	if ts == nil {
		ts = tipSet{}
	}
	id := b.Cid()
	ts[id.String()] = id
	tsbp[key] = ts
}

func keyForParentSet(parents []*cid.Cid) string {
	// TODO: who should be responsible for sorting the cid set?
	// Proposal: cid.Set should just always be sorted and we should use that here, not a slice.
	var k string
	for _, id := range parents {
		k += id.String()
	}
	return k
}

// TODO: We'll need more than just the Cid for each matching block, so define a new struct here
// that is a subset of types.Block that has just the state needed for EC.
type tipSet map[string]*cid.Cid
