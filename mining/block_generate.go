package mining

// Block generation is part of the logic of the DefaultWorker.
// 'generate' is that function that actually creates a new block from a base
// TipSet using the DefaultWorker's many utilities.

import (
	"context"

	errors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
)

// Generate returns a new block created from the messages in the pool.
func (w *DefaultWorker) Generate(ctx context.Context, baseTipSet consensus.TipSet, ticket types.Signature, nullBlockCount uint64) (*types.Block, error) {
	stateTree, err := w.getStateTree(ctx, baseTipSet)
	if err != nil {
		return nil, errors.Wrap(err, "get state tree")
	}

	if !w.powerTable.HasPower(ctx, stateTree, w.blockstore, w.minerAddr) {
		return nil, errors.Errorf("bad miner address, miner must store files before mining: %s", w.minerAddr)
	}

	wNum, wDenom, err := w.getWeight(ctx, baseTipSet)
	if err != nil {
		return nil, errors.Wrap(err, "get weight")
	}

	nonce, err := core.NextNonce(ctx, stateTree, w.messagePool, address.NetworkAddress)
	if err != nil {
		return nil, errors.Wrap(err, "next nonce")
	}

	baseHeight, err := baseTipSet.Height()
	if err != nil {
		return nil, errors.Wrap(err, "get base tip set height")
	}

	blockHeight := baseHeight + nullBlockCount + 1
	rewardMsg := types.NewMessage(address.NetworkAddress, w.minerAddr, nonce, types.NewAttoFILFromFIL(1000), "", nil)
	srewardMsg := &types.SignedMessage{
		Message:   *rewardMsg,
		Signature: nil,
	}

	pending := w.messagePool.Pending()
	messages := make([]*types.SignedMessage, len(pending)+1)
	messages[0] = srewardMsg // Reward message must come first since this is a part of the consensus rules.
	copy(messages[1:], core.OrderMessagesByNonce(w.messagePool.Pending()))

	vms := vm.NewStorageMap(w.blockstore)
	res, err := w.applyMessages(ctx, messages, stateTree, vms, types.NewBlockHeight(blockHeight))

	if err != nil {
		return nil, errors.Wrap(err, "generate apply messages")
	}

	newStateTreeCid, err := stateTree.Flush(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "generate flush state tree")
	}

	if err = vms.Flush(); err != nil {
		return nil, errors.Wrap(err, "generate flush vm storage map")
	}

	var receipts []*types.MessageReceipt
	for _, r := range res.Results {
		receipts = append(receipts, r.Receipt)
	}

	next := &types.Block{
		Miner:             w.minerAddr,
		Height:            types.Uint64(blockHeight),
		Messages:          res.SuccessfulMessages,
		MessageReceipts:   receipts,
		Parents:           baseTipSet.ToSortedCidSet(),
		ParentWeightNum:   types.Uint64(wNum),
		ParentWeightDenom: types.Uint64(wDenom),
		StateRoot:         newStateTreeCid,
		Ticket:            ticket,
	}

	var rewardSuccessful bool
	for _, msg := range res.SuccessfulMessages {
		if msg == srewardMsg {
			rewardSuccessful = true
		}
	}
	if !rewardSuccessful {
		return nil, errors.New("mining reward message failed")
	}
	// Mining reward message succeeded -- side effects okay below this point.

	for _, msg := range res.SuccessfulMessages {
		mc, err := msg.Cid()
		if err == nil {
			w.messagePool.Remove(mc)
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
			w.messagePool.Remove(mc)
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
