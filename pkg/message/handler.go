package message

import (
	"context"

	"github.com/filecoin-project/venus/pkg/block"
)

// HeadHandler wires up new head tipset handling to the message inbox and outbox.
type HeadHandler struct {
	// Inbox and outbox exported for testing.
	Inbox  *Inbox
	Outbox *Outbox
	chain  chainProvider
}

// NewHeadHandler build a new new-head handler.
func NewHeadHandler(inbox *Inbox, outbox *Outbox, chain chainProvider) *HeadHandler {
	return &HeadHandler{inbox, outbox, chain}
}

// HandleNewHead computes the chain delta implied by a new head and updates the inbox and outbox.
func (h *HeadHandler) HandleNewHead(ctx context.Context, droppedBlocks, applyBlocks []*block.TipSet) error {
	if err := h.Outbox.HandleNewHead(ctx, droppedBlocks, applyBlocks); err != nil {
		log.Errorf("updating outbound message queue for tipset %d, prev %d: %s", len(applyBlocks), len(droppedBlocks), err)
	}

	if err := h.Inbox.HandleNewHead(ctx, droppedBlocks, applyBlocks); err != nil {
		log.Errorf("updating message pool for tipset %d, prev %d: %s", len(applyBlocks), len(droppedBlocks), err)
	}

	return nil
}
