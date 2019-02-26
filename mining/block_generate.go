package mining

// Block generation is part of the logic of the DefaultWorker.
// 'generate' is that function that actually creates a new block from a base
// TipSet using the DefaultWorker's many utilities.

import (
	"context"
	"time"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
)

// Generate returns a new block created from the messages in the pool.
func (w *DefaultWorker) Generate(ctx context.Context,
	baseTipSet types.TipSet,
	ticket types.Signature,
	proof proofs.PoStProof,
	nullBlockCount uint64) (*types.Block, error) {

	generateTimer := time.Now()
	defer func() {
		log.Infof("[TIMER] DefaultWorker.Generate baseTipset: %s - elapsed time: %s", baseTipSet.String(), time.Since(generateTimer).Round(time.Millisecond))
	}()

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

	ancestors, err := w.getAncestors(ctx, baseTipSet, types.NewBlockHeight(blockHeight))
	if err != nil {
		return nil, errors.Wrap(err, "get base tip set ancestors")
	}

	pending := w.messageSource.Pending()
	mq := NewMessageQueue(pending)
	messages := mq.Drain()

	vms := vm.NewStorageMap(w.blockstore)
	res, err := w.processor.ApplyMessagesAndPayRewards(ctx, stateTree, vms, messages, w.minerAddr, types.NewBlockHeight(blockHeight), ancestors)
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

	for i, msg := range res.PermanentFailures {
		// We will not be able to apply this message in the future because the error was permanent.
		// Therefore, we will remove it from the MessagePool now.
		// There might be better places to do this, such as wherever successful messages are removed
		// from the pool, or by posting the failure to an event bus to be handled async.
		log.Infof("permanent ApplyMessage failure, [%s] (%s)", msg, res.PermanentErrors[i])
		mc, err := msg.Cid()
		if err == nil {
			w.messageSource.Remove(mc)
		} else {
			log.Warningf("failed to get CID from message", err)
		}
	}

	for i, msg := range res.TemporaryFailures {
		// We might be able to apply this message in the future because the error was temporary.
		// Therefore, we will leave it in the MessagePool for now.

		log.Infof("temporary ApplyMessage failure, [%s] (%s)", msg, res.TemporaryErrors[i])
	}

	return next, nil
}
