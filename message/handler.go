package message

import (
	"context"

	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/types"
)

// HeadHandler wires up new head tipset handling to the message inbox and outbox.
type HeadHandler struct {
	// Inbox and outbox exported for testing.
	Inbox  *Inbox
	Outbox *Outbox
	chain  chainProvider

	prevHead types.TipSet
}

// NewHeadHandler build a new new-head handler.
func NewHeadHandler(inbox *Inbox, outbox *Outbox, chain chainProvider, head types.TipSet) *HeadHandler {
	return &HeadHandler{inbox, outbox, chain, head}
}

// HandleNewHead computes the chain delta implied by a new head and updates the inbox and outbox.
func (h *HeadHandler) HandleNewHead(ctx context.Context, newHead types.TipSet) error {
	if !newHead.Defined() {
		log.Warning("received empty tipset, ignoring")
		return nil
	}
	if newHead.Equals(h.prevHead) {
		log.Warningf("received non-new head tipset, ignoring %s", newHead.Key())
		return nil
	}

	oldTips, newTips, err := chain.CollectTipsToCommonAncestor(ctx, h.chain, h.prevHead, newHead)
	if err != nil {
		return errors.Errorf("traversing chain with new head %s, prev %s: %s", newHead.Key(), h.prevHead.Key(), err)
	}
	if err := h.Outbox.HandleNewHead(ctx, oldTips, newTips); err != nil {
		log.Errorf("updating outbound message queue for tipset %s, prev %s: %s", newHead.Key(), h.prevHead.Key(), err)
	}
	if err := h.Inbox.HandleNewHead(ctx, oldTips, newTips); err != nil {
		log.Errorf("updating message pool for tipset %s, prev %s: %s", newHead.Key(), h.prevHead.Key(), err)
	}

	h.prevHead = newHead
	return nil
}
