package mining

import (
	"context"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
)

// processBlockFunc is a signature that makes it easier to test Generate().
type processBlockFunc func(context.Context, *types.Block, types.StateTree) ([]*types.MessageReceipt, error)

// processBlock is the functoin that does the block processing.
var processBlock = core.ProcessBlock

// BlockGenerator is the primary interface for blockGenerator.
type BlockGenerator interface {
	Generate(context.Context, *types.Block, types.StateTree) (*types.Block, error)
}

// NewBlockGenerator returns a new BlockGenerator.
func NewBlockGenerator(mp *core.MessagePool) BlockGenerator {
	return &blockGenerator{mp}
}

// blockGenerator generates new blocks for inclusion in the chain.
type blockGenerator struct {
	Mp *core.MessagePool
}

// Generate returns a new block created from the messages in the
// pool. It does not remove them.
func (b blockGenerator) Generate(ctx context.Context, p *types.Block, st types.StateTree) (*types.Block, error) {
	child := &types.Block{
		Height:   p.Height + 1,
		Messages: b.Mp.Pending(),
	}
	if err := child.AddParent(*p); err != nil {
		return nil, err
	}

	receipts, err := processBlock(ctx, child, st)
	if err != nil {
		return nil, err
	}

	child.MessageReceipts = receipts

	newStCid, err := st.Flush(ctx)
	if err != nil {
		return nil, err
	}
	child.StateRoot = newStCid

	return child, nil
}
