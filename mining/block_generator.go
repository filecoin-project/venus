package mining

import (
	"context"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
)

// ProcessBlockFunc is a signature that makes it easier to test Generate().
type ProcessBlockFunc func(context.Context, *types.Block, types.StateTree) error

var ProcessBlock = core.ProcessBlock

// BlockGeneratorInterface is the primary interface for a BlockGenerator. It's
// used by the tests of higher-level code to create a mock or fake BlockGenerators
// as a dependency.
type BlockGeneratorInterface interface {
	Generate(context.Context, *types.Block, types.StateTree) (*types.Block, error)
}

// BlockGenerator generates new blocks for inclusion in the chain.
type BlockGenerator struct {
	Mp *core.MessagePool
}

// Generate returns a new block created from the messages in the
// pool. It does not remove them.
func (b BlockGenerator) Generate(ctx context.Context, p *types.Block, st types.StateTree) (*types.Block, error) {
	child := &types.Block{
		Height:   p.Height + 1,
		Messages: b.Mp.Pending(),
	}
	if err := child.AddParent(*p); err != nil {
		return nil, err
	}

	if err := ProcessBlock(ctx, child, st); err != nil {
		return nil, err
	}
	newStCid, err := st.Flush(ctx)
	if err != nil {
		return nil, err
	}
	child.StateRoot = newStCid

	return child, nil
}
