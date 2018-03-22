package core

import (
	"context"

	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	hamt "gx/ipfs/QmdtiofXbibTe6Day9ii5zjBZpSRm8vhfoerrNuY3sAQ7e/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/types"
)

// MustPut stores the thingy in the store or panics if it cannot.
func MustPut(store *hamt.CborIpldStore, thingy interface{}) *cid.Cid {
	cid, err := store.Put(context.Background(), thingy)
	if err != nil {
		panic(err)
	}
	return cid
}

// MustAdd adds the given messages to the messagepool or panics if it
// cannot.
func MustAdd(p *MessagePool, msgs ...*types.Message) {
	for _, m := range msgs {
		if _, err := p.Add(m); err != nil {
			panic(err)
		}
	}
}

// NewChainWithMessages creates a chain of blocks containing the given messages
// and stores them in the given store. Note the msg arguments are slices of
// messages -- each slice goes into a successive block.
func NewChainWithMessages(store *hamt.CborIpldStore, root *types.Block, msgSets ...[]*types.Message) []*types.Block {
	parent := root
	blocks := []*types.Block{}
	if parent != nil {
		MustPut(store, parent)
		blocks = append(blocks, parent)
	}

	for _, msgs := range msgSets {
		child := &types.Block{Messages: msgs}
		if parent != nil {
			child.Parent = parent.Cid()
			child.Height = parent.Height + 1
		}
		MustPut(store, child)
		blocks = append(blocks, child)
		parent = child
	}

	return blocks
}
