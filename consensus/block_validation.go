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
	Now() int64
	BlockTime() time.Duration
}

type DefaultBlockValidationClock struct {
	blkTime time.Duration
}

func NewDefaultBlockValidationClock(bt time.Duration) *DefaultBlockValidationClock {
	return &DefaultBlockValidationClock{
		blkTime: bt,
	}
}

func (ebc *DefaultBlockValidationClock) Now() int64 {
	return time.Now().Unix()
}

func (ebc *DefaultBlockValidationClock) BlockTime() time.Duration {
	return ebc.blkTime
}

type DefaultBlockValidator struct {
	clock BlockValidationClock
}

func NewDefaultBlockValidator(c BlockValidationClock) *DefaultBlockValidator {
	return &DefaultBlockValidator{
		clock: c,
	}
}

func (dv *DefaultBlockValidator) ValidateSemantic(ctx context.Context, child, parent *types.Block) error {
	// TODO validate timestamp
	return nil
}

func (dv *DefaultBlockValidator) ValidateSyntax(ctx context.Context, blk *types.Block) error {
	if !blk.StateRoot.Defined() {
		return fmt.Errorf("block has nil StateRoot")
	}
	// TODO validate timestamp
	// TODO validate block signature
	return nil
}
