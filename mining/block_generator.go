package mining

import (
	"context"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"

	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

// GetStateTree is a function that gets a state tree by cid. It's
// its own function to facilitate testing.
type GetStateTree func(context.Context, *cid.Cid) (types.StateTree, error)

// BlockGenerator is the primary interface for blockGenerator.
type BlockGenerator interface {
	Generate(context.Context, *types.Block, types.Address) (*types.Block, error)
}

// NewBlockGenerator returns a new BlockGenerator.
func NewBlockGenerator(messagePool *core.MessagePool, getStateTree GetStateTree, processBlock core.Processor) BlockGenerator {
	return &blockGenerator{
		messagePool:  messagePool,
		getStateTree: getStateTree,
		processBlock: processBlock,
	}
}

// blockGenerator generates new blocks for inclusion in the chain.
type blockGenerator struct {
	messagePool  *core.MessagePool
	getStateTree GetStateTree
	processBlock core.Processor
}

// Generate returns a new block created from the messages in the pool.
func (b blockGenerator) Generate(ctx context.Context, baseBlock *types.Block, rewardAddress types.Address) (*types.Block, error) {
	stateTree, err := b.getStateTree(ctx, baseBlock.StateRoot)
	if err != nil {
		return nil, err
	}

	next := &types.Block{
		Height:   baseBlock.Height + 1,
		Messages: b.messagePool.Pending(),
	}
	if err := next.AddParent(*baseBlock); err != nil {
		return nil, err
	}

	rewardMsg := types.NewMessage(core.NetworkAccount, rewardAddress, types.NewTokenAmount(1000), "", nil)
	next.Messages = append(next.Messages, rewardMsg)

	// TODO(fritz) processBlock bails as soon as it sees a
	// message failure. Change this to skip the message
	// and surface it somehow, to order by nonce, etc.
	receipts, err := b.processBlock(ctx, next, stateTree)
	if err != nil {
		return nil, err
	}

	next.MessageReceipts = receipts

	newStateTreeCid, err := stateTree.Flush(ctx)
	if err != nil {
		return nil, err
	}
	next.StateRoot = newStateTreeCid

	return next, nil
}
