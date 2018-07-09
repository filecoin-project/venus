package mining

import (
	"context"

	logging "gx/ipfs/QmQCqiR5F3NeJRr7LuWq8i8FgtT65ypZw5v9V6Es6nwFBD/go-log"
	xerrors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

var log = logging.Logger("mining")

// GetStateTree is a function that gets the aggregate state tree of a TipSet. It's
// its own function to facilitate testing.
type GetStateTree func(context.Context, core.TipSet) (state.Tree, error)

// GetWeight is a function that calculates the weight of a TipSet.  It's its
// own function to facilitate testing.
type GetWeight func(context.Context, core.TipSet) (uint64, error)

// BlockGenerator is the primary interface for blockGenerator.
type BlockGenerator interface {
	Generate(context.Context, core.TipSet, types.Signature, uint64, types.Address) (*types.Block, error)
}

// NewBlockGenerator returns a new BlockGenerator.
func NewBlockGenerator(messagePool *core.MessagePool, getStateTree GetStateTree, getWeight GetWeight, applyMessages miningApplier) BlockGenerator {
	return &blockGenerator{
		messagePool:   messagePool,
		getStateTree:  getStateTree,
		getWeight:     getWeight,
		applyMessages: applyMessages,
	}
}

type miningApplier func(ctx context.Context, messages []*types.Message, st state.Tree, bh *types.BlockHeight) (core.ApplyMessagesResponse, error)

// blockGenerator generates new blocks for inclusion in the chain.
type blockGenerator struct {
	messagePool   *core.MessagePool
	getStateTree  GetStateTree
	getWeight     GetWeight
	applyMessages miningApplier
}

// Generate returns a new block created from the messages in the pool.
func (b blockGenerator) Generate(ctx context.Context, baseTipSet core.TipSet, ticket types.Signature, nullBlockCount uint64, rewardAddress types.Address) (*types.Block, error) {
	stateTree, err := b.getStateTree(ctx, baseTipSet)
	if err != nil {
		return nil, err
	}

	weight, err := b.getWeight(ctx, baseTipSet)
	if err != nil {
		return nil, err
	}

	nonce, err := core.NextNonce(ctx, stateTree, b.messagePool, address.NetworkAddress)
	if err != nil {
		return nil, err
	}

	baseHeight, err := baseTipSet.Height()
	if err != nil {
		return nil, err
	}
	blockHeight := baseHeight + nullBlockCount + 1
	rewardMsg := types.NewMessage(address.NetworkAddress, rewardAddress, nonce, types.NewAttoFILFromFIL(1000), "", nil)
	pending := b.messagePool.Pending()
	messages := make([]*types.Message, len(pending)+1)
	messages[0] = rewardMsg // Reward message must come first since this is a part of the consensus rules.
	copy(messages[1:], core.OrderMessagesByNonce(b.messagePool.Pending()))

	res, err := b.applyMessages(ctx, messages, stateTree, types.NewBlockHeight(blockHeight))
	if err != nil {
		return nil, err
	}

	newStateTreeCid, err := stateTree.Flush(ctx)
	if err != nil {
		return nil, err
	}

	var receipts []*types.MessageReceipt
	for _, r := range res.Results {
		receipts = append(receipts, r.Receipt)
	}

	next := &types.Block{
		Miner:           rewardAddress,
		Height:          blockHeight,
		Messages:        res.SuccessfulMessages,
		MessageReceipts: receipts,
		Parents:         baseTipSet.ToSortedCidSet(),
		ParentWeight:    weight,
		StateRoot:       newStateTreeCid,
		Ticket:          ticket,
	}

	var rewardSuccessful bool
	for _, msg := range res.SuccessfulMessages {
		if msg == rewardMsg {
			rewardSuccessful = true
		}
	}
	if !rewardSuccessful {
		return nil, xerrors.New("mining reward message failed")
	}
	// Mining reward message succeeded -- side effects okay below this point.

	for _, msg := range res.SuccessfulMessages {
		mc, err := msg.Cid()
		if err == nil {
			b.messagePool.Remove(mc)
		}
	}

	// TODO: Should we really be pruning the message pool here at all? Maybe this should happen elsewhere.
	for _, msg := range res.PermanentFailures {
		// We will not be able to apply this message in the future because the error was permanent.
		// Therefore, we will remove it from the MessagePool now.
		mc, err := msg.Cid()
		log.Infof("permanent ApplyMessage failure, [%S]", mc.String())
		// Intentionally not handling error case, since it just means we won't be able to remove from pool.
		if err == nil {
			b.messagePool.Remove(mc)
		}
	}

	for _, msg := range res.TemporaryFailures {
		// We might be able to apply this message in the future because the error was temporary.
		// Therefore, we will leave it in the MessagePool for now.
		mc, _ := msg.Cid()
		log.Infof("temporary ApplyMessage failure, [%S]", mc.String())
	}

	return next, nil
}
