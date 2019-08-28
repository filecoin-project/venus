package consensus

import (
	"context"

	"github.com/filecoin-project/go-filecoin/types"
)

// MessageSyntaxValidator defines an interface used to validate collections
// of messages and receipts syntax
type MessageSyntaxValidator interface {
	ValidateMessagesSyntax(ctx context.Context, messages []*types.SignedMessage) error
	ValidateReceiptsSyntax(ctx context.Context, receipts []*types.MessageReceipt) error
}

// TODO: Define a message semantics validator if necessary
// See: https://github.com/filecoin-project/go-filecoin/issues/3312

// DefaultMessageSyntaxValidator implements the MessageSyntaxValidator interface.
type DefaultMessageSyntaxValidator struct {
}

// NewDefaultMessageSyntaxValidator returns a new DefaultMessageSyntaxValidator.
func NewDefaultMessageSyntaxValidator() *DefaultMessageSyntaxValidator {
	return &DefaultMessageSyntaxValidator{}
}

// ValidateMessagesSyntax validates a set of messages are correctly formed.
// TODO: Create a real implementation
// See: https://github.com/filecoin-project/go-filecoin/issues/3312
func (dv *DefaultMessageSyntaxValidator) ValidateMessagesSyntax(ctx context.Context, messages []*types.SignedMessage) error {
	return nil
}

// ValidateReceiptsSyntax validates a set of receipts are correctly formed.
// TODO: Create a real implementation
// See: https://github.com/filecoin-project/go-filecoin/issues/3312
func (dv *DefaultMessageSyntaxValidator) ValidateReceiptsSyntax(ctx context.Context, receipts []*types.MessageReceipt) error {
	return nil
}
