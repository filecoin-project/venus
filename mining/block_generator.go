package mining

import (
	"context"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
)

// ProcessBlockFunc is a signature that makes it easier to test Generate().
type ProcessBlockFunc func(context.Context, *types.Block) error

// FlushTreeFunc is a signature that makes it easier to test Generate().
type FlushTreeFunc func(context.Context) (*cid.Cid, error)

// BlockGeneratorInterface is the primary interface for a BlockGenerator. It's
// used by the tests of higher-level code to create a mock or fake BlockGenerators
// as a dependency.
type BlockGeneratorInterface interface {
	Generate(context.Context, *types.Block, ProcessBlockFunc, FlushTreeFunc) (*types.Block, error)
}

// BlockGenerator generates new blocks for inclusion in the chain.
type BlockGenerator struct {
	Mp *core.MessagePool
}

// Generate returns a new block created from the messages in the
// pool. It does not remove them. Passing in functions to do the
// processing and flushing enables us to hide those details from this
// level, making it easy to test.
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
