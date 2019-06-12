package consensus

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/clock"
	"github.com/filecoin-project/go-filecoin/types"
)

var (
	// ErrTooSoon is returned when a block is generated too soon after its parent.
	ErrTooSoon = errors.New("block was generated too soon")
	// ErrInvalidHeight is returned when a block has an invliad height.
	ErrInvalidHeight = errors.New("block has invalid height")
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
		return ErrInvalidHeight
	}

	// check that child is appropriately delayed from its parents including
	// null blocks.
	// TODO replace check on height when #2222 lands
	limit := uint64(pmin) + uint64(dv.BlockTime().Seconds())*(uint64(child.Height)-ph)
	if uint64(child.Timestamp) < limit {
		//return errors.Wrapf(ErrTooSoon, "limit: %d, childTs: %d", limit, child.Timestamp)
		return ErrTooSoon
	}
	return nil
}

// ValidateSyntax validates a single block is correctly formed.
func (dv *DefaultBlockValidator) ValidateSyntax(ctx context.Context, blk *types.Block) error {
	if uint64(blk.Timestamp) > uint64(dv.Now().Unix()) {
		return fmt.Errorf("block generate in future")
	}
	if !blk.StateRoot.Defined() {
		return fmt.Errorf("block has nil StateRoot")
	}
	if blk.Miner.Empty() {
		return fmt.Errorf("block has nil miner address")
	}
	if len(blk.Ticket) == 0 {
		return fmt.Errorf("block has nil ticket")
	}
	// TODO validate block signature: 1054
	return nil
}

// BlockTime returns the block time the DefaultBlockValidator uses to validate
/// blocks against.
func (dv *DefaultBlockValidator) BlockTime() time.Duration {
	return dv.blockTime
}
