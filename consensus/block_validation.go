package consensus

import (
	"context"
	"fmt"
	"time"

	"github.com/filecoin-project/go-filecoin/clock"
	"github.com/filecoin-project/go-filecoin/types"
)

// BlockValidator defines an interface used to validate a blocks syntax and
// semantics.
type BlockValidator interface {
	BlockSemanticValidator
	BlockSyntaxValidator
}

// BlockSemanticValidator defines an interface used to validate a blocks
// semantics.
type BlockSemanticValidator interface {
	ValidateSemantic(ctx context.Context, child *types.Block, parents *types.TipSet) error
}

// BlockSyntaxValidator defines an interface used to validate a blocks
// syntax.
type BlockSyntaxValidator interface {
	ValidateSyntax(ctx context.Context, blk *types.Block) error
}

// DefaultBlockValidator implements the BlockValidator interface.
type DefaultBlockValidator struct {
	clock.Clock
	blockTime time.Duration
}

// NewDefaultBlockValidator returns a new DefaultBlockValidator. It uses `blkTime`
// to validate blocks and uses the DefaultBlockValidationClock.
func NewDefaultBlockValidator(blkTime time.Duration, c clock.Clock) *DefaultBlockValidator {
	return &DefaultBlockValidator{
		Clock:     c,
		blockTime: blkTime,
	}
}

// ValidateSemantic validates a block is correctly derived from its parent.
func (dv *DefaultBlockValidator) ValidateSemantic(ctx context.Context, child *types.Block, parents *types.TipSet) error {
	pmin, err := parents.MinTimestamp()
	if err != nil {
		return err
	}

	ph, err := parents.Height()
	if err != nil {
		return err
	}

	if uint64(child.Height) <= ph {
		return fmt.Errorf("block %s has invalid height %d", child.Cid().String(), child.Height)
	}

	// check that child is appropriately delayed from its parents including
	// null blocks.
	// TODO replace check on height when #2222 lands
	limit := uint64(pmin) + uint64(dv.BlockTime().Seconds())*(uint64(child.Height)-ph)
	if uint64(child.Timestamp) < limit {
		return fmt.Errorf("block %s with timestamp %d generated too far past parent, expected timestamp < %d", child.Cid().String(), child.Timestamp, limit)
	}
	return nil
}

// ValidateSyntax validates a single block is correctly formed.
func (dv *DefaultBlockValidator) ValidateSyntax(ctx context.Context, blk *types.Block) error {
	// TODO special handling for genesis block
	// figure out in: https://github.com/filecoin-project/go-filecoin/issues/3121
	if blk.Height == 0 {
		return nil
	}
	now := uint64(dv.Now().Unix())
	if uint64(blk.Timestamp) > now {
		return fmt.Errorf("block %s with timestamp %d generate in future at time %d", blk.Cid().String(), blk.Timestamp, now)
	}
	if !blk.StateRoot.Defined() {
		return fmt.Errorf("block %s has nil StateRoot", blk.Cid().String())
	}
	if blk.Miner.Empty() {
		return fmt.Errorf("block %s has nil miner address", blk.Cid().String())
	}
	if len(blk.Ticket) == 0 {
		return fmt.Errorf("block %s has nil ticket", blk.Cid().String())
	}
	// TODO validate block signature: 1054
	return nil
}

// BlockTime returns the block time the DefaultBlockValidator uses to validate
/// blocks against.
func (dv *DefaultBlockValidator) BlockTime() time.Duration {
	return dv.blockTime
}
