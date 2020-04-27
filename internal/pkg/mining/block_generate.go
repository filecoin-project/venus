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
	"github.com/filecoin-project/go-filecoin/internal/pkg/drand"
	e "github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// Generate returns a new block created from the messages in the pool.
// The resulting output is not empty: it has either a block or an error.
func (w *DefaultWorker) Generate(
	ctx context.Context,
	baseTipSet block.TipSet,
	ticket block.Ticket,
	electionProof crypto.VRFPi,
	nullBlockCount abi.ChainEpoch,
	posts []block.PoStProof,
	drandEntries []*drand.Entry,
) Output {

	generateTimer := time.Now()
	defer func() {
		log.Infof("[TIMER] DefaultWorker.Generate baseTipset: %s - elapsed time: %s", baseTipSet.String(), time.Since(generateTimer).Round(time.Millisecond))
	}()

	weight, err := w.getWeight(ctx, baseTipSet)
	if err != nil {
		return NewOutputErr(errors.Wrap(err, "get weight"))
	}

	baseHeight, err := baseTipSet.Height()
	if err != nil {
		return NewOutputErr(errors.Wrap(err, "get base tip set height"))
	}

	blockHeight := baseHeight + nullBlockCount + 1

	// Construct list of message candidates for inclusion.
	// These messages will be processed, and those that fail excluded from the block.
	pending := w.messageSource.Pending()
	mq := NewMessageQueue(pending)
	candidateMsgs := orderMessageCandidates(mq.Drain())
	candidateMsgs = w.filterPenalizableMessages(ctx, candidateMsgs)

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
		return NewOutputErr(errors.Wrap(err, "could not aggregate bls messages"))
	}

	// Persist messages to ipld storage
	txMetaCid, err := w.messageStore.StoreMessages(ctx, secpAccepted, unwrappedBLSMessages)
	if err != nil {
		return NewOutputErr(errors.Wrap(err, "error persisting messages"))
	}

	// get tipset state root and receipt root
	baseStateRoot, err := w.tsMetadata.GetTipSetStateRoot(baseTipSet.Key())
	if err != nil {
		return NewOutputErr(errors.Wrapf(err, "error retrieving state root for tipset %s", baseTipSet.Key().String()))
	}

	baseReceiptRoot, err := w.tsMetadata.GetTipSetReceiptsRoot(baseTipSet.Key())
	if err != nil {
		return NewOutputErr(errors.Wrapf(err, "error retrieving receipt root for tipset %s", baseTipSet.Key().String()))
	}

	// Set the block timestamp to be exactly the start of the target epoch, regardless of the current time.
	// The real time might actually be much later than this if catching up from a pause in chain progress.
	epochStartTime := w.clock.StartTimeOfEpoch(blockHeight)

	next := &block.Block{
		Miner:           w.minerAddr,
		Height:          blockHeight,
		DrandEntries:    drandEntries,
		ElectionProof:   &crypto.ElectionProof{VRFProof: electionProof},
		Messages:        e.NewCid(txMetaCid),
		MessageReceipts: e.NewCid(baseReceiptRoot),
		Parents:         baseTipSet.Key(),
		ParentWeight:    weight,
		PoStProofs:      posts,
		StateRoot:       e.NewCid(baseStateRoot),
		Ticket:          ticket,
		Timestamp:       uint64(epochStartTime.Unix()),
		BLSAggregateSig: &blsAggregateSig,
	}

	view, err := w.api.PowerStateView(baseTipSet.Key())
	if err != nil {
		return NewOutputErr(errors.Wrapf(err, "failed to read state view"))
	}
	_, workerAddr, err := view.MinerControlAddresses(ctx, w.minerAddr)
	if err != nil {
		return NewOutputErr(errors.Wrap(err, "failed to read workerAddr during block generation"))
	}
	workerSigningAddr, err := view.AccountSignerAddress(ctx, workerAddr)
	if err != nil {
		return NewOutputErr(errors.Wrap(err, "failed to convert worker address to signing address"))
	}
	blockSig, err := w.workerSigner.SignBytes(ctx, next.SignatureData(), workerSigningAddr)
	if err != nil {
		return NewOutputErr(errors.Wrap(err, "failed to sign block"))
	}
	next.BlockSig = &blockSig

	return NewOutput(next, blsAccepted, secpAccepted)
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

func (w *DefaultWorker) filterPenalizableMessages(ctx context.Context, messages []*types.SignedMessage) []*types.SignedMessage {
	var goodMessages []*types.SignedMessage
	for _, msg := range messages {
		err := w.penaltyChecker.PenaltyCheck(ctx, &msg.Message)
		if err != nil {
			mCid, _ := msg.Cid()
			log.Debugf("Msg: %s excluded in block because penalized with err %s", mCid, err)
			continue
		}
		goodMessages = append(goodMessages, msg)
	}
	return goodMessages
}
