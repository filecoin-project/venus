package testhelpers

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
)

// FakeBlockValidator passes everything as valid
type FakeBlockValidator struct{}

// NewFakeBlockValidator createas a FakeBlockValidator that passes everything as valid.
func NewFakeBlockValidator() *FakeBlockValidator {
	return &FakeBlockValidator{}
}

// ValidateHeaderSemantic does nothing.
func (fbv *FakeBlockValidator) ValidateHeaderSemantic(ctx context.Context, child *types.BlockHeader, parents types.TipSet) error {
	return nil
}

// ValidateSyntax does nothing.
func (fbv *FakeBlockValidator) ValidateSyntax(ctx context.Context, blk *types.BlockHeader) error {
	return nil
}

// ValidateMessagesSyntax does nothing
func (fbv *FakeBlockValidator) ValidateMessagesSyntax(ctx context.Context, messages []*types.SignedMessage) error {
	return nil
}

// ValidateUnsignedMessagesSyntax does nothing
func (fbv *FakeBlockValidator) ValidateUnsignedMessagesSyntax(ctx context.Context, messages []*types.Message) error {
	return nil
}

// ValidateReceiptsSyntax does nothing
func (fbv *FakeBlockValidator) ValidateReceiptsSyntax(ctx context.Context, receipts []types.MessageReceipt) error {
	return nil
}

// StubBlockValidator is a mockable block validator.
type StubBlockValidator struct {
	syntaxStubs map[cid.Cid]error
}

// NewStubBlockValidator creates a StubBlockValidator that allows errors to configured
// for blocks passed to the Validate* methods.
func NewStubBlockValidator() *StubBlockValidator {
	return &StubBlockValidator{
		syntaxStubs: make(map[cid.Cid]error),
	}
}

// ValidateSyntax return nil or error for stubbed block `blk`.
func (mbv *StubBlockValidator) ValidateBlockMsg(ctx context.Context, blk *types.BlockMsg) pubsub.ValidationResult {
	if mbv.syntaxStubs[blk.Header.Cid()] == nil {
		return pubsub.ValidationAccept
	}
	return pubsub.ValidationReject
}

// StubSyntaxValidationForBlock stubs an error when the ValidateSyntax is called
// on the with the given block.
func (mbv *StubBlockValidator) StubSyntaxValidationForBlock(blk *types.BlockHeader, err error) {
	mbv.syntaxStubs[blk.Cid()] = err
}
