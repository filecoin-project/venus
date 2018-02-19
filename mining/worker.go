package mining

import (
	"context"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
)

// AddNewBlockFunc is a signature that enables us to hide implementation
// details from the Worker and makes it easier to test.
type AddNewBlockFunc func(context.Context, *types.Block) error

// Worker mines. If successful it passes the new block to AddNewBlock()
// and returns its cid.
type Worker struct {
	BlockGenerator BlockGeneratorInterface
	AddNewBlock    AddNewBlockFunc
}

// NewWorker instantiates a new
func NewWorker(blockGenerator BlockGeneratorInterface, addNewBlock AddNewBlockFunc) *Worker {
	return &Worker{blockGenerator, addNewBlock}
}

// Mine attempts to mine one block. Returns the cid of the new block, if any.
func (w *Worker) Mine(ctx context.Context, cur *types.Block, tree types.StateTreeInterface) (*cid.Cid, error) {
	processBlock := func(ctx context.Context, b *types.Block) error {
		return core.ProcessBlock(ctx, b, tree)
	}
	flushTree := func(ctx context.Context) (*cid.Cid, error) {
		return tree.Flush(ctx)
	}
	next, err := w.BlockGenerator.Generate(ctx, cur, processBlock, flushTree)
	if err != nil {
		return nil, err
	}
	return next.Cid(), w.AddNewBlock(ctx, next)
}
