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

// mkChild creates a new block with parent, blk, and supplied nonce.
func mkChild(blk *types.Block, nonce uint64) *types.Block {
	return &types.Block{
		Parent:          blk.Cid(),
		Height:          blk.Height + 1,
		Nonce:           nonce,
		StateRoot:       blk.StateRoot,
		Messages:        []*types.Message{},
		MessageReceipts: []*types.MessageReceipt{},
	}
}

// AddChain creates and processes new, empty chain of length, beginning from blk.
func AddChain(ctx context.Context, processNewBlock NewBlockProcessor, blk *types.Block, length int) (*types.Block, error) {
	l := uint64(length)
	for i := uint64(0); i < l; i++ {
		blk = mkChild(blk, i)
		_, err := processNewBlock(ctx, blk)
		if err != nil {
			return nil, err
		}
	}
	return blk, nil
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
