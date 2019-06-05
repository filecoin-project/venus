package consensus

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-filecoin/plumbing/clock"
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
	clock.BlockClock
}

// NewDefaultBlockValidator returns a new DefaultBlockValidator. It uses `blkTime`
// to validate blocks and uses the DefaultBlockValidationClock.
func NewDefaultBlockValidator(c clock.BlockClock) *DefaultBlockValidator {
	return &DefaultBlockValidator{
		BlockClock: c,
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

	// check that child is appropriately delayed from its parents including
	// null blocks.
	// TODO replace check on height when #2222 lands
	limit := uint64(pmin) + uint64(dv.BlockTime().Seconds())*uint64(uint64(child.Height)-ph)
	if uint64(child.Timestamp) < limit {
		return fmt.Errorf("block was generated too soon")
	}

	// #2886
	return nil
}

// ValidateSyntax validates a single block is correctly formed.
func (dv *DefaultBlockValidator) ValidateSyntax(ctx context.Context, blk *types.Block) error {
	if !blk.StateRoot.Defined() {
		return fmt.Errorf("block has nil StateRoot")
	}

	if blk.Timestamp > types.Uint64(dv.EpochSeconds()) {
		return fmt.Errorf("block was generated too far in the future")
	}

	// TODO validate block signature
	// #2886
	return nil
}
