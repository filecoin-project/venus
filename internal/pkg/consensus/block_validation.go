package consensus

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-state-types/abi"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/pkg/errors"

	builtin0 "github.com/filecoin-project/specs-actors/actors/builtin"
	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/clock"
	"github.com/filecoin-project/venus/internal/pkg/state"
	"github.com/filecoin-project/venus/internal/pkg/types"
	"github.com/filecoin-project/venus/internal/pkg/vm/gas"
	//"github.com/filecoin-project/venus/internal/pkg/vm/gas"
	//"github.com/filecoin-project/go-state-types/abi"
)

var log = logging.Logger("consensus")

type messageStore interface {
	LoadMetaMessages(context.Context, cid.Cid) ([]*types.SignedMessage, []*types.UnsignedMessage, error)
	LoadReceipts(context.Context, cid.Cid) ([]types.MessageReceipt, error)
}

type chainState interface {
	GetActorAt(context.Context, block.TipSetKey, address.Address) (*types.Actor, error)
	GetTipSet(block.TipSetKey) (*block.TipSet, error)
	GetTipSetStateRoot(context.Context, block.TipSetKey) (cid.Cid, error)
	StateView(block.TipSetKey, abi.ChainEpoch) (*state.View, error)
	GetBlock(context.Context, cid.Cid) (*block.Block, error)
}

// BlockValidator defines an interface used to validate a blocks syntax and
// semantics.
type BlockValidator interface {
	BlockSemanticValidator
	BlockSyntaxValidator
}

// SyntaxValidator defines and interface used to validate block's syntax and the
// syntax of constituent messages
type SyntaxValidator interface {
	BlockSyntaxValidator
	MessageSyntaxValidator
}

// BlockSemanticValidator defines an interface used to validate a blocks
// semantics.
type BlockSemanticValidator interface {
	ValidateHeaderSemantic(ctx context.Context, child *block.Block, parents block.TipSet) error
	ValidateMessagesSemantic(ctx context.Context, child *block.Block, parents block.TipSetKey) error
}

// BlockSyntaxValidator defines an interface used to validate a blocks
// syntax.
type BlockSyntaxValidator interface {
	ValidateSyntax(ctx context.Context, blk *block.Block) error
}

// MessageSyntaxValidator defines an interface used to validate a message's
// syntax.
type MessageSyntaxValidator interface {
	ValidateSignedMessageSyntax(ctx context.Context, smsg *types.SignedMessage) error
	ValidateUnsignedMessageSyntax(ctx context.Context, msg *types.UnsignedMessage) error
}

// DefaultBlockValidator implements the BlockValidator interface.
type DefaultBlockValidator struct {
	clock.ChainEpochClock
	ms messageStore
	cs chainState
}

// WrappedSyntaxValidator implements syntax validator interface
type WrappedSyntaxValidator struct {
	BlockSyntaxValidator
	MessageSyntaxValidator
}

// NewDefaultBlockValidator returns a new DefaultBlockValidator. It uses `blkTime`
// to validate blocks and uses the DefaultBlockValidationClock.
func NewDefaultBlockValidator(c clock.ChainEpochClock, m messageStore, cs chainState) *DefaultBlockValidator {
	return &DefaultBlockValidator{
		ChainEpochClock: c,
		ms:              m,
		cs:              cs,
	}
}

// NotFutureBlock errors if the block belongs to a future epoch according to
// the chain clock.
func (dv *DefaultBlockValidator) NotFutureBlock(b *block.Block) error {
	//currentEpoch := dv.EpochAtTime(dv.Now())
	//if b.Height > currentEpoch {
	//	return fmt.Errorf("block %s with timestamp %d generate in future epoch %d", b.Cid().String(), b.Timestamp, b.Height)
	//}

	now := uint64(dv.Now().Unix())
	if b.Timestamp > now+AllowableClockDriftSecs {
		return xerrors.Errorf("block was from the future (now=%d, blk=%d): temporal error", now, b.Timestamp)
	}
	if b.Timestamp > now {
		log.Warn("Got block from the future, but within threshold", b.Timestamp, dv.Now().Unix())
	}

	return nil
}

// ValidateHeaderSemantic checks validation conditions on a header that can be
// checked given only the parent header.
func (dv *DefaultBlockValidator) ValidateHeaderSemantic(ctx context.Context, child *block.Block, parents *block.TipSet) error {
	ph, err := parents.Height()
	if err != nil {
		return err
	}

	if child.Height <= ph {
		return fmt.Errorf("block %s has invalid height %d", child.Cid().String(), child.Height)
	}

	return nil
}

func (dv *DefaultBlockValidator) validateMessage(msg *types.UnsignedMessage, expectedCallSeqNum map[address.Address]uint64, fromActor *types.Actor) error {
	callSeq, ok := expectedCallSeqNum[msg.From]
	if !ok {
		callSeq = fromActor.CallSeqNum
	}

	// ensure message is in the correct order
	if callSeq != msg.CallSeqNum {
		return fmt.Errorf("callseqnum (%d) out of order (expected %d) from %s", msg.CallSeqNum, callSeq, msg.From)
	}

	expectedCallSeqNum[msg.From] = callSeq + 1
	return nil
}

// ValidateFullSemantic checks validation conditions on a block's messages that don't require message execution.
func (dv *DefaultBlockValidator) ValidateMessagesSemantic(ctx context.Context, child *block.Block, parents block.TipSetKey) error {
	// validate call sequence numbers
	//secpMsgs, blsMsgs, err := dv.ms.LoadMessages(ctx, child.Messages.Cid)
	//if err != nil {
	//	return errors.Wrapf(err, "block validation failed loading message list %s for block %s", child.Messages, child.Cid())
	//}
	//
	//expectedCallSeqNum := map[address.Address]uint64{}
	//for _, msg := range blsMsgs {
	//	msgCid, err := msg.Cid()
	//	if err != nil {
	//		return err
	//	}
	//
	//	from, err := dv.getAndValidateFromActor(ctx, msg, parents)
	//	if err != nil {
	//		return errors.Wrapf(err, "from actor %s for message %s of block %s invalid", msg.From, msgCid, child.Cid())
	//	}
	//
	//	err = dv.validateMessage(msg, expectedCallSeqNum, from)
	//	if err != nil {
	//		return errors.Wrapf(err, "message %s of block %s invalid", msgCid, child.Cid())
	//	}
	//}
	//
	//for _, msg := range secpMsgs {
	//	msgCid, err := msg.Cid()
	//	if err != nil {
	//		return err
	//	}
	//
	//	from, err := dv.getAndValidateFromActor(ctx, &msg.Message, parents)
	//	if err != nil {
	//		return errors.Wrapf(err, "from actor %s for message %s of block %s invalid", msg.Message.From, msgCid, child.Cid())
	//	}
	//
	//	err = dv.validateMessage(&msg.Message, expectedCallSeqNum, from)
	//	if err != nil {
	//		return errors.Wrapf(err, "message %s of block %s invalid", msgCid, child.Cid())
	//	}
	//}

	// ToDo 同步时存储落后于验证,TipSetMetadata尚未被存储?
	secpMsgs, blsMsgs, err := dv.ms.LoadMetaMessages(ctx, child.Messages.Cid)
	if err != nil {
		return errors.Wrapf(err, "block validation failed loading message list %s for block %s", child.Messages, child.Cid())
	}

	pl := gas.PricelistByEpoch(child.Height)
	var sumGasLimit int64
	checkMsg := func(msg types.ChainMsg) error {
		m := msg.VMMessage()

		if m.ChainLength() > 32*1024 {
			log.Warnf("message is too large! (%dB)", m.ChainLength())
			return xerrors.Errorf("message is too large! (%dB)", m.ChainLength())
		}

		if m.To == address.Undef {
			return xerrors.Errorf("local message has invalid destination address")
		}

		//if !m.Value.LessThan(types.TotalFilecoinInt) {
		//	return xerrors.Errorf("value-too-high")
		//}

		minGas := pl.OnChainMessage(msg.ChainLength())
		if err := m.ValidForBlockInclusion(minGas.Total()); err != nil {
			return err
		}

		sumGasLimit += int64(m.GasLimit)
		if sumGasLimit > types.BlockGasLimit {
			return xerrors.Errorf("block gas limit exceeded")
		}

		return nil
	}

	for i, m := range blsMsgs {
		if err := checkMsg(m); err != nil {
			return xerrors.Errorf("block had invalid bls message at index %d: %w", i, err)
		}
	}

	for i, m := range secpMsgs {
		if err := checkMsg(m); err != nil {
			return xerrors.Errorf("block had invalid secpk message at index %d: %w", i, err)
		}
	}

	//callSeqNums := make(map[address.Address]uint64)
	//checkMsg := func(msg types.ChainMsg) error {
	//	m := msg.VMMessage()
	//
	//	// Phase 2: (Partial) semantic validation:
	//	// the sender exists and is an account actor, and the nonces make sense
	//	if _, ok := callSeqNums[m.From]; !ok {
	//		// `GetActor` does not validate that this is an account actor.
	//		act, err := dv.getAndValidateFromActor(ctx, m, parents)
	//		if err != nil {
	//			log.Warnf("failed to get actor for %s of parents %s, err: %s", m.From, parents, err.Error())
	//			return nil
	//		}
	//
	//		if !act.IsAccountActor() {
	//			return xerrors.New("Sender must be an account actor")
	//		}
	//		callSeqNums[m.From] = act.CallSeqNum
	//	}
	//
	//	if callSeqNums[m.From] != m.CallSeqNum {
	//		return xerrors.Errorf("wrong nonce (exp: %d, got: %d)", callSeqNums[m.From], m.CallSeqNum)
	//	}
	//	callSeqNums[m.From]++
	//
	//	return nil
	//}
	//
	//for i, m := range blsMsgs {
	//	if err := checkMsg(m); err != nil {
	//		return xerrors.Errorf("block had invalid bls message at index %d: %w", i, err)
	//	}
	//}
	//
	//for i, m := range secpMsgs {
	//	if err := checkMsg(m); err != nil {
	//		return xerrors.Errorf("block had invalid secpk message at index %d: %w", i, err)
	//	}
	//
	//	// Signature Validator
	//	view, err := dv.cs.AccountStateView(parents)
	//	if err != nil {
	//		return errors.Wrapf(err, "failed to load state at %v", parents)
	//	}
	//
	//	sigValidator := state.NewSignatureValidator(view)
	//
	//	if err := sigValidator.ValidateMessageSignature(ctx, m); err != nil {
	//		return errors.Wrap(err, fmt.Errorf("invalid signature by sender over message data").Error())
	//	}
	//}

	return nil
}

func (dv *DefaultBlockValidator) getAndValidateFromActor(ctx context.Context, msg *types.UnsignedMessage, parents block.TipSetKey) (*types.Actor, error) {
	actor, err := dv.cs.GetActorAt(ctx, parents, msg.From)
	if err != nil {
		return nil, err
	}

	// ensure actor is an account actor
	if !actor.Code.Equals(builtin0.AccountActorCodeID) && actor.Code.Equals(builtin2.AccountActorCodeID) {
		return nil, errors.New("sent from non-account actor")
	}

	return actor, nil
}

// ValidateSyntax validates a single block is correctly formed.
func (dv *DefaultBlockValidator) ValidateSyntax(ctx context.Context, blk *block.Block) (err error) {
	if blk.Height == 0 {
		return nil
	}

	err = dv.NotFutureBlock(blk)
	if err != nil {
		return err
	}

	if !blk.StateRoot.Defined() {
		return fmt.Errorf("block %s has nil StateRoot", blk.Cid())
	}

	if blk.Miner.Empty() {
		return fmt.Errorf("block %s has nil miner address", blk.Cid())
	}

	if len(blk.Ticket.VRFProof) == 0 {
		return fmt.Errorf("block %s has nil ticket", blk.Cid())
	}

	if blk.BlockSig == nil {
		return fmt.Errorf("block %s has nil signature", blk.Cid())
	}

	return nil
}
