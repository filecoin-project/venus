package mining

// Block generation is part of the logic of the DefaultWorker.
// 'generate' is that function that actually creates a new block from a base
// TipSet using the DefaultWorker's many utilities.

import (
	"context"
	"github.com/filecoin-project/go-filecoin/proofs"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
)

// Generate returns a new block created from the messages in the pool.
func (w *DefaultWorker) Generate(ctx context.Context,
	baseTipSet consensus.TipSet,
	ticket types.Signature,
	proof proofs.PoStProof,
	nullBlockCount uint64) (*types.Block, error) {

	stateTree, err := w.getStateTree(ctx, baseTipSet)
	if err != nil {
		return nil, errors.Wrap(err, "get state tree")
	}

	if !w.powerTable.HasPower(ctx, stateTree, w.blockstore, w.minerAddr) {
		return nil, errors.Errorf("bad miner address, miner must store files before mining: %s", w.minerAddr)
	}

	weight, err := w.getWeight(ctx, baseTipSet)

	if err != nil {
		return nil, errors.Wrap(err, "get weight")
	}

	baseHeight, err := baseTipSet.Height()
	if err != nil {
		return nil, errors.Wrap(err, "get base tip set height")
	}

	blockHeight := baseHeight + nullBlockCount + 1

	pending := w.messagePool.Pending()
	messages := make([]*types.SignedMessage, len(pending))

	copy(messages, core.OrderMessagesByNonce(pending))

	vms := vm.NewStorageMap(w.blockstore)
	res, err := w.processor.ApplyMessagesAndPayRewards(ctx, stateTree, vms, messages, w.minerAddr, types.NewBlockHeight(blockHeight))
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
		Miner:           w.minerAddr,
		Height:          types.Uint64(blockHeight),
		Messages:        res.SuccessfulMessages,
		MessageReceipts: receipts,
		Parents:         baseTipSet.ToSortedCidSet(),
		ParentWeight:    types.Uint64(weight),
		Proof:           proof,
		StateRoot:       newStateTreeCid,
		Ticket:          ticket,
	}

	// TODO: Should we really be pruning the message pool here at all? Maybe this should happen elsewhere.
	for i, msg := range res.PermanentFailures {
		// We will not be able to apply this message in the future because the error was permanent.
		// Therefore, we will remove it from the MessagePool now.
		log.Infof("permanent ApplyMessage failure, [%s] (%s)", msg, res.PermanentErrors[i])
		// Intentionally not handling error case, since it just means we won't be able to remove from pool.
		mc, err := msg.Cid()
		if err == nil {
			w.messagePool.Remove(mc)
		}
	}

	for i, msg := range res.TemporaryFailures {
		// We might be able to apply this message in the future because the error was temporary.
		// Therefore, we will leave it in the MessagePool for now.

		log.Infof("temporary ApplyMessage failure, [%s] (%s)", msg, res.TemporaryErrors[i])
	}

	return next, nil
}
