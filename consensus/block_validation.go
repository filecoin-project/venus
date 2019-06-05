package consensus

import (
	"context"
	"fmt"
	"time"

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

// BlockValidationClock defines an interface for fetching unix epoch time.
type BlockValidationClock interface {
	EpochSeconds() uint64
}

// DefaultBlockValidationClock implements BlockValidationClock using the
// Go time package.
type DefaultBlockValidationClock struct{}

// NewDefaultBlockValidationClock returns a DefaultBlockValidationClock.
func NewDefaultBlockValidationClock() *DefaultBlockValidationClock {
	return &DefaultBlockValidationClock{}
}

// EpochSeconds returns Unix time, the number of seconds elapsed since January 1, 1970 UTC.
// The result does not depend on location.
func (ebc *DefaultBlockValidationClock) EpochSeconds() uint64 {
	return uint64(time.Now().Unix())
}

// DefaultBlockValidator implements the BlockValidator interface.
type DefaultBlockValidator struct {
	clock clock.BlockClock
}

// NewDefaultBlockValidator returns a new DefaultBlockValidator. It uses `blkTime`
// to validate blocks and uses the DefaultBlockValidationClock.
func NewDefaultBlockValidator(c clock.BlockClock) *DefaultBlockValidator {
	return &DefaultBlockValidator{
		clock: c,
	}
}

// ValidateSemantic validates a block is correctly derived from its parent.
func (dv *DefaultBlockValidator) ValidateSemantic(ctx context.Context, child *types.Block, parents *types.TipSet) error {
	// TODO validate timestamp
	// #2886
	return nil
}

// ValidateSyntax validates a single block is correctly formed.
func (dv *DefaultBlockValidator) ValidateSyntax(ctx context.Context, blk *types.Block) error {
	if !blk.StateRoot.Defined() {
		return fmt.Errorf("block has nil StateRoot")
	}
	// TODO validate timestamp
	// TODO validate block signature
	// #2886
	return nil
}

// BlockTime returns the block time the DefaultBlockValidator uses to validate
/// blocks against.
func (dv *DefaultBlockValidator) BlockTime() time.Duration {
	return dv.clock.BlockTime()
}
