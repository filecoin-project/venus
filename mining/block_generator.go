package mining

import (
	"context"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
)

// BlockGenerator generates new blocks for inclusion in the chain.
type BlockGenerator struct {
	Mp *core.MessagePool
}

// ProcessBlockFunc is a signature that makes it easier to test Generate().
type ProcessBlockFunc func(context.Context, *types.Block) error

// FlushTreeFunc is a signature that makes it easier to test Generate().
type FlushTreeFunc func(context.Context) (*cid.Cid, error)

// Generate returns a new block created from the messages in the
// pool. It does not remove them.
// Note: we pass in the state tree to avoid having to need a store
// here to load the tree -- easier for testing.
func (b BlockGenerator) Generate(ctx context.Context, p *types.Block, processBlock ProcessBlockFunc, flushTree FlushTreeFunc) (*types.Block, error) {
	child := &types.Block{
		Height:   p.Height + 1,
		Messages: b.Mp.Pending(),
	}
	if err := child.AddParent(*p); err != nil {
		return nil, err
	}
	if err := processBlock(ctx, child); err != nil {
		return nil, err
	}
	newCid, err := flushTree(ctx)
	if err != nil {
		return nil, err
	}
	child.StateRoot = newCid

	return child, nil
}
