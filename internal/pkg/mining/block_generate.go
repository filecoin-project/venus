package mining

// Block generation is part of the logic of the DefaultWorker.
// 'generate' is that function that actually creates a new block from a base
// TipSet using the DefaultWorker's many utilities.

import (
	"context"
	"time"

	"github.com/pkg/errors"

	"github.com/filecoin-project/go-bls-sigs"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

// Generate returns a new block created from the messages in the pool.
func (w *DefaultWorker) Generate(ctx context.Context,
	baseTipSet block.TipSet,
	ticket block.Ticket,
	electionProof block.VRFPi,
	nullBlockCount uint64) (*block.Block, error) {

	generateTimer := time.Now()
	defer func() {
		log.Infof("[TIMER] DefaultWorker.Generate baseTipset: %s - elapsed time: %s", baseTipSet.String(), time.Since(generateTimer).Round(time.Millisecond))
	}()

	stateTree, err := w.getStateTree(ctx, baseTipSet)
	if err != nil {
		return nil, errors.Wrap(err, "get state tree")
	}

	powerTable, err := w.getPowerTable(ctx, baseTipSet.Key())
	if err != nil {
		return nil, errors.Wrap(err, "get power table")
	}

	if !powerTable.HasPower(ctx, w.minerAddr) {
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

	// Construct list of message candidates for inclusion.
	// These messages will be processed, and those that fail excluded from the block.
	pending := w.messageSource.Pending()
	mq := NewMessageQueue(pending)
	candidateMsgs := orderMessageCandidates(mq.Drain())

	// run state transition to learn which messages are valid
	vms := vm.NewStorageMap(w.blockstore)
	results, err := w.processor.ApplyMessagesAndPayRewards(ctx, stateTree, vms, types.UnwrapSigned(candidateMsgs),
		w.minerOwnerAddr, types.NewBlockHeight(blockHeight), ancestors)
	if err != nil {
		return nil, errors.Wrap(err, "generate apply messages")
	}

	var blsAccepted []*types.SignedMessage
	var secpAccepted []*types.SignedMessage

	// Align the results with the candidate signed messages to accumulate the messages lists
	// to include in the block, and handle failed messages.
	for i, r := range results {
		msg := candidateMsgs[i]
		if r.Failure == nil {
			if msg.Message.From.Protocol() == address.BLS {
				blsAccepted = append(blsAccepted, msg)
			} else {
				secpAccepted = append(secpAccepted, msg)
			}
		} else if r.FailureIsPermanent {
			// Remove message that can never succeed from the message pool now.
			// There might be better places to do this, such as wherever successful messages are removed
			// from the pool, or by posting the failure to an event bus to be handled async.
			log.Infof("permanent ApplyMessage failure, [%s] (%s)", msg, r.Failure)
			mc, err := msg.Cid()
			if err == nil {
				w.messageSource.Remove(mc)
			} else {
				log.Warnf("failed to get CID from message", err)
			}
		} else {
			// This message might succeed in the future, so leave it in the pool for now.
			log.Infof("temporary ApplyMessage failure, [%s] (%s)", msg, r.Failure)
		}
	}

	// Create an aggregage signature for messages
	unwrappedBLSMessages, blsAggregateSig, err := aggregateBLS(blsAccepted)
	if err != nil {
		return nil, errors.Wrap(err, "could not aggregate bls messages")
	}

	// Persist messages to ipld storage
	txMeta, err := w.messageStore.StoreMessages(ctx, secpAccepted, unwrappedBLSMessages)
	if err != nil {
		return nil, errors.Wrap(err, "error persisting messages")
	}

	// get tipset state root and receipt root
	baseStateRoot, err := w.tsMetadata.GetTipSetStateRoot(baseTipSet.Key())
	if err != nil {
		return nil, errors.Wrapf(err, "error retrieving state root for tipset %s", baseTipSet.Key().String())
	}

	baseReceiptRoot, err := w.tsMetadata.GetTipSetReceiptsRoot(baseTipSet.Key())
	if err != nil {
		return nil, errors.Wrapf(err, "error retrieving receipt root for tipset %s", baseTipSet.Key().String())
	}

	next := &block.Block{
		Miner:           w.minerAddr,
		Height:          types.Uint64(blockHeight),
		Messages:        txMeta,
		MessageReceipts: baseReceiptRoot,
		Parents:         baseTipSet.Key(),
		ParentWeight:    types.Uint64(weight),
		ElectionProof:   electionProof,
		StateRoot:       baseStateRoot,
		Ticket:          ticket,
		Timestamp:       types.Uint64(w.clock.Now().Unix()),
		BLSAggregateSig: blsAggregateSig,
	}
	workerAddr, err := w.api.MinerGetWorkerAddress(ctx, w.minerAddr, baseTipSet.Key())
	if err != nil {
		return nil, errors.Wrap(err, "failed to read workerAddr during block generation")
	}
	next.BlockSig, err = w.workerSigner.SignBytes(next.SignatureData(), workerAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign block")
	}

	return next, nil
}

func aggregateBLS(blsMessages []*types.SignedMessage) ([]*types.UnsignedMessage, types.Signature, error) {
	sigs := []bls.Signature{}
	unwrappedMsgs := []*types.UnsignedMessage{}
	for _, msg := range blsMessages {
		// unwrap messages
		unwrappedMsgs = append(unwrappedMsgs, &msg.Message)
		sig := msg.Signature

		// store message signature as bls signature
		blsSig := bls.Signature{}
		copy(blsSig[:], sig)
		sigs = append(sigs, blsSig)
	}
	blsAggregateSig := bls.Aggregate(sigs)
	if blsAggregateSig == nil {
		return []*types.UnsignedMessage{}, types.Signature{}, errors.New("could not aggregate signatures")
	}
	return unwrappedMsgs, blsAggregateSig[:], nil
}

// When a block is validated, BLS messages are processed first, so for simplicity all BLS
// messages are considered first here too.
func orderMessageCandidates(messages []*types.SignedMessage) []*types.SignedMessage {
	blsMessages := []*types.SignedMessage{}
	secpMessages := []*types.SignedMessage{}

	for _, m := range messages {
		if m.Message.From.Protocol() == address.BLS {
			blsMessages = append(blsMessages, m)
		} else {
			secpMessages = append(secpMessages, m)
		}
	}
	return append(blsMessages, secpMessages...)
}
