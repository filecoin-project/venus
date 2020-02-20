package consensus

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
)

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
	ValidateSemantic(ctx context.Context, child *block.Block, parents block.TipSet) error
}

// BlockSyntaxValidator defines an interface used to validate a blocks
// syntax.
type BlockSyntaxValidator interface {
	ValidateSyntax(ctx context.Context, blk *block.Block) error
}

// MessageSyntaxValidator defines an interface used to validate collections
// of messages and receipts syntax
type MessageSyntaxValidator interface {
	ValidateMessagesSyntax(ctx context.Context, messages []*types.SignedMessage) error
	ValidateUnsignedMessagesSyntax(ctx context.Context, messages []*types.UnsignedMessage) error
	// TODO: Remove receipt validation when they're no longer fetched, #3489
	ValidateReceiptsSyntax(ctx context.Context, receipts []vm.MessageReceipt) error
}

// DefaultBlockValidator implements the BlockValidator interface.
type DefaultBlockValidator struct {
	clock.ChainEpochClock
}

// NewDefaultBlockValidator returns a new DefaultBlockValidator. It uses `blkTime`
// to validate blocks and uses the DefaultBlockValidationClock.
func NewDefaultBlockValidator(c clock.ChainEpochClock) *DefaultBlockValidator {
	return &DefaultBlockValidator{
		ChainEpochClock: c,
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

// ValidateSemantic checks validation conditions on a header that can be
// checked given only the parent header.
func (dv *DefaultBlockValidator) ValidateSemantic(ctx context.Context, child *block.Block, parents block.TipSet) error {
	ph, err := parents.Height()
	if err != nil {
		return err
	}

	if child.Height <= ph {
		return fmt.Errorf("block %s has invalid height %d", child.Cid().String(), child.Height)
	}

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
		return fmt.Errorf("block %s has nil StateRoot", blk.Cid().String())
	}
	if blk.Miner.Empty() {
		return fmt.Errorf("block %s has nil miner address", blk.Cid().String())
	}
	if len(blk.Ticket.VRFProof) == 0 {
		return fmt.Errorf("block %s has nil ticket", blk.Cid().String())
	}

	return nil
}

// ValidateMessagesSyntax validates a set of messages are correctly formed.
// TODO: Create a real implementation
// See: https://github.com/filecoin-project/go-filecoin/issues/3312
func (dv *DefaultBlockValidator) ValidateMessagesSyntax(ctx context.Context, messages []*types.SignedMessage) error {
	return nil
}

// ValidateUnsignedMessagesSyntax validates a set of messages are correctly formed.
// TODO: Create a real implementation
// See: https://github.com/filecoin-project/go-filecoin/issues/3312
func (dv *DefaultBlockValidator) ValidateUnsignedMessagesSyntax(ctx context.Context, messages []*types.UnsignedMessage) error {
	return nil
}

// ValidateReceiptsSyntax validates a set of receipts are correctly formed.
// TODO: Create a real implementation
// See: https://github.com/filecoin-project/go-filecoin/issues/3312
func (dv *DefaultBlockValidator) ValidateReceiptsSyntax(ctx context.Context, receipts []vm.MessageReceipt) error {
	return nil
}
