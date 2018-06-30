package core

import (
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
