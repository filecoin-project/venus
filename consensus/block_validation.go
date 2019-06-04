package consensus

import (
	"context"
	"fmt"
	"time"

	"github.com/filecoin-project/go-filecoin/types"
)

type BlockValidator interface {
	BlockSemanticValidator
	BlockSyntaxValidator
}

type BlockSemanticValidator interface {
	ValidateSemantic(ctx context.Context, child, parent *types.Block) error
}

type BlockSyntaxValidator interface {
	ValidateSyntax(ctx context.Context, blk *types.Block) error
}

type BlockValidationClock interface {
	EpochSeconds() uint64
}

type DefaultBlockValidationClock struct{}

func NewDefaultBlockValidationClock() *DefaultBlockValidationClock {
	return &DefaultBlockValidationClock{}
}

func (ebc *DefaultBlockValidationClock) EpochSeconds() uint64 {
	return uint64(time.Now().Unix())
}

type DefaultBlockValidator struct {
	clock     BlockValidationClock
	blockTime time.Duration
}

func NewDefaultBlockValidator(blkTime time.Duration) *DefaultBlockValidator {
	return &DefaultBlockValidator{
		clock:     NewDefaultBlockValidationClock(),
		blockTime: blkTime,
	}
}

func (dv *DefaultBlockValidator) ValidateSemantic(ctx context.Context, child, parent *types.Block) error {
	// TODO validate timestamp
	// #2886
	return nil
}

func (dv *DefaultBlockValidator) ValidateSyntax(ctx context.Context, blk *types.Block) error {
	if !blk.StateRoot.Defined() {
		return fmt.Errorf("block has nil StateRoot")
	}
	// TODO validate timestamp
	// TODO validate block signature
	// #2886
	return nil
}

func (dv *DefaultBlockValidator) BlockTime() time.Duration {
	return dv.blockTime
}
