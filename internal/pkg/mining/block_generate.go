package mining

// Block generation is part of the logic of the DefaultWorker.
// 'generate' is that function that actually creates a new block from a base
// TipSet using the DefaultWorker's many utilities.

import (
	"context"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/pkg/errors"

	bls "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	e "github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// Generate returns a new block created from the messages in the pool.
func (w *DefaultWorker) Generate(
	ctx context.Context,
	baseTipSet block.TipSet,
	ticket block.Ticket,
	nullBlockCount abi.ChainEpoch,
	ePoStInfo block.EPoStInfo,
) (*block.Block, error) {

	generateTimer := time.Now()
	defer func() {
		log.Infof("[TIMER] DefaultWorker.Generate baseTipset: %s - elapsed time: %s", baseTipSet.String(), time.Since(generateTimer).Round(time.Millisecond))
	}()

	powerTable, err := w.getPowerTable(baseTipSet.Key())
	if err != nil {
		return nil, errors.Wrap(err, "get power table")
	}

	hasPower, err := powerTable.HasClaimedPower(ctx, w.minerAddr)
	if err != nil {
		return nil, err
	}
	if !hasPower {
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

	// Construct list of message candidates for inclusion.
	// These messages will be processed, and those that fail excluded from the block.
	pending := w.messageSource.Pending()
	mq := NewMessageQueue(pending)
	candidateMsgs := orderMessageCandidates(mq.Drain())

	// Dragons: ask something to select and order messages to include

	var blsAccepted []*types.SignedMessage
	var secpAccepted []*types.SignedMessage

	// Align the results with the candidate signed messages to accumulate the messages lists
	// to include in the block, and handle failed messages.
	for _, msg := range candidateMsgs {
		if msg.Message.From.Protocol() == address.BLS {
			blsAccepted = append(blsAccepted, msg)
		} else {
			secpAccepted = append(secpAccepted, msg)
		}

	}

	// Create an aggregage signature for messages
	unwrappedBLSMessages, blsAggregateSig, err := aggregateBLS(blsAccepted)
	if err != nil {
		return nil, errors.Wrap(err, "could not aggregate bls messages")
	}

	// Persist messages to ipld storage
	txMetaCid, err := w.messageStore.StoreMessages(ctx, secpAccepted, unwrappedBLSMessages)
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

	now := w.clock.Now()
	next := &block.Block{
		Miner:           w.minerAddr,
		Height:          blockHeight,
		Messages:        e.NewCid(txMetaCid),
		MessageReceipts: e.NewCid(baseReceiptRoot),
		Parents:         baseTipSet.Key(),
		ParentWeight:    weight,
		EPoStInfo:       ePoStInfo,
		StateRoot:       e.NewCid(baseStateRoot),
		Ticket:          ticket,
		Timestamp:       uint64(now.Unix()),
		BLSAggregateSig: blsAggregateSig,
	}

	view, err := w.api.PowerStateView(baseTipSet.Key())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read state view")
	}
	_, workerAddr, err := view.MinerControlAddresses(ctx, w.minerAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read workerAddr during block generation")
	}
	next.BlockSig, err = w.workerSigner.SignBytes(next.SignatureData(), workerAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign block")
	}

	return next, nil
}

func aggregateBLS(blsMessages []*types.SignedMessage) ([]*types.UnsignedMessage, crypto.Signature, error) {
	var sigs []bls.Signature
	var unwrappedMsgs []*types.UnsignedMessage
	for _, msg := range blsMessages {
		// unwrap messages
		unwrappedMsgs = append(unwrappedMsgs, &msg.Message)
		if msg.Signature.Type != crypto.SigTypeBLS {
			return []*types.UnsignedMessage{}, crypto.Signature{}, errors.New("non-BLS message signature")
		}

		// store message signature as bls signature
		blsSig := bls.Signature{}
		copy(blsSig[:], msg.Signature.Data)
		sigs = append(sigs, blsSig)
	}
	blsAggregateSig := bls.Aggregate(sigs)
	if blsAggregateSig == nil {
		return []*types.UnsignedMessage{}, crypto.Signature{}, errors.New("could not aggregate signatures")
	}
	return unwrappedMsgs, crypto.Signature{
		Type: crypto.SigTypeBLS,
		Data: blsAggregateSig[:],
	}, nil
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
