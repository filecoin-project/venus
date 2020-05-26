package consensus

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/specs-actors/actors/builtin"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

type messageStore interface {
	LoadMessages(context.Context, cid.Cid) ([]*types.SignedMessage, []*types.UnsignedMessage, error)
	LoadReceipts(context.Context, cid.Cid) ([]vm.MessageReceipt, error)
}

type chainState interface {
	GetActorAt(ctx context.Context, tipKey block.TipSetKey, addr address.Address) (*actor.Actor, error)
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
	currentEpoch := dv.EpochAtTime(dv.Now())
	if b.Height > currentEpoch {
		return fmt.Errorf("block %s with timestamp %d generate in future epoch %d", b.Cid().String(), b.Timestamp, b.Height)
	}
	return nil
}

// TimeMatchesEpoch errors if the epoch and time don't match according to the
// chain clock.
func (dv *DefaultBlockValidator) TimeMatchesEpoch(b *block.Block) error {
	earliestExpected, latestExpected := dv.EpochRangeAtTimestamp(b.Timestamp)
	blockEpoch := b.Height
	if (blockEpoch < earliestExpected) || (blockEpoch > latestExpected) {
		return fmt.Errorf(
			"block %s with timestamp %d generated in wrong epoch %d, expected epoch in range [%d, %d]",
			b.Cid().String(),
			b.Timestamp,
			b.Height,
			earliestExpected,
			latestExpected,
		)
	}
	return nil
}

// ValidateHeaderSemantic checks validation conditions on a header that can be
// checked given only the parent header.
func (dv *DefaultBlockValidator) ValidateHeaderSemantic(ctx context.Context, child *block.Block, parents block.TipSet) error {
	ph, err := parents.Height()
	if err != nil {
		return err
	}

	if child.Height <= ph {
		return fmt.Errorf("block %s has invalid height %d", child.Cid().String(), child.Height)
	}

	return nil
}

// ValidateFullSemantic checks validation conditions on a block's messages that don't require message execution.
func (dv *DefaultBlockValidator) ValidateMessagesSemantic(ctx context.Context, child *block.Block, parents block.TipSetKey) error {
	// validate call sequence numbers
	secpMsgs, blsMsgs, err := dv.ms.LoadMessages(ctx, child.Messages.Cid)
	if err != nil {
		return errors.Wrapf(err, "block validation failed loading message list %s for block %s", child.Messages, child.Cid())
	}

	expectedCallSeqNum := map[address.Address]uint64{}
	for _, msg := range blsMsgs {
		msgCid, err := msg.Cid()
		if err != nil {
			return err
		}

		from, err := dv.getAndValidateFromActor(ctx, msg, parents)
		if err != nil {
			return errors.Wrapf(err, "from actor %s for message %s of block %s invalid", msg.From, msgCid, child.Cid())
		}

		err = dv.validateMessage(msg, expectedCallSeqNum, from)
		if err != nil {
			return errors.Wrapf(err, "message %s of block %s invalid", msgCid, child.Cid())
		}
	}

	for _, msg := range secpMsgs {
		msgCid, err := msg.Cid()
		if err != nil {
			return err
		}

		from, err := dv.getAndValidateFromActor(ctx, &msg.Message, parents)
		if err != nil {
			return errors.Wrapf(err, "from actor %s for message %s of block %s invalid", msg.Message.From, msgCid, child.Cid())
		}

		err = dv.validateMessage(&msg.Message, expectedCallSeqNum, from)
		if err != nil {
			return errors.Wrapf(err, "message %s of block %s invalid", msgCid, child.Cid())
		}
	}

	return nil
}

func (dv *DefaultBlockValidator) getAndValidateFromActor(ctx context.Context, msg *types.UnsignedMessage, parents block.TipSetKey) (*actor.Actor, error) {
	actor, err := dv.cs.GetActorAt(ctx, parents, msg.From)
	if err != nil {
		return nil, err
	}

	// ensure actor is an account actor
	if !actor.Code.Equals(builtin.AccountActorCodeID) {
		return nil, errors.New("sent from non-account actor")
	}

	return actor, nil
}

func (dv *DefaultBlockValidator) validateMessage(msg *types.UnsignedMessage, expectedCallSeqNum map[address.Address]uint64, fromActor *actor.Actor) error {
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

// ValidateSyntax validates a single block is correctly formed.
// TODO this is an incomplete implementation #3277
func (dv *DefaultBlockValidator) ValidateSyntax(ctx context.Context, blk *block.Block) error {
	// TODO special handling for genesis block #3121
	if blk.Height == 0 {
		return nil
	}
	err := dv.NotFutureBlock(blk)
	if err != nil {
		return err
	}
	err = dv.TimeMatchesEpoch(blk)
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

	//TODO: validate all the messages syntax

	return nil
}
