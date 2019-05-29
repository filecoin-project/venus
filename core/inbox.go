package core

import (
	"context"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/ipfs/go-cid"
)

// InboxMaxAgeTipsets is maximum age (in non-empty tipsets) to permit messages to stay in the pool after reception.
// It should be a little shorter than the outbox max age so that messages expire from mining
// pools a little before the sender gives up on them.
const InboxMaxAgeTipsets = 6

// Inbox maintains a pool of received messages.
type Inbox struct {
	// The pool storing received messages.
	pool *MessagePool
	// Maximum age of a pool message.
	maxAgeTipsets uint

	chain InboxChainProvider
}

// InboxChainProvider provides chain access for updating the message pool in response to new heads.
// Exported for testing.
type InboxChainProvider interface {
	chain.BlockProvider
	BlockHeight() (uint64, error)
}

// NewInbox constructs a new inbox.
func NewInbox(pool *MessagePool, maxAgeRounds uint, chain InboxChainProvider) *Inbox {
	return &Inbox{pool: pool, maxAgeTipsets: maxAgeRounds, chain: chain}
}

// Add adds a message to the pool, tagged with the current block height.
// An error probably means the message failed to validate,
// but it could indicate a more serious problem with the system.
func (ib *Inbox) Add(ctx context.Context, msg *types.SignedMessage) (cid.Cid, error) {
	blockTime, err := ib.chain.BlockHeight()
	if err != nil {
		return cid.Undef, err
	}

	return ib.pool.Add(ctx, msg, blockTime)
}

// Pool returns the inbox's message pool.
func (ib *Inbox) Pool() *MessagePool {
	return ib.pool
}

// HandleNewHead updates the message pool in response to a new head tipset.
// This removes messages from the pool that are found in the newly adopted chain and adds back
// those from the removed chain (if any) that do not appear in the new chain.
// We think that the right model for keeping the message pool up to date is
// to think about it like a garbage collector.
func (ib *Inbox) HandleNewHead(ctx context.Context, oldHead, newHead types.TipSet) error {
	oldBlocks, newBlocks, err := CollectBlocksToCommonAncestor(ctx, ib.chain, oldHead, newHead)
	if err != nil {
		return err
	}

	// Add all message from the old blocks to the message pool, so they can be mined again.
	for _, blk := range oldBlocks {
		for _, msg := range blk.Messages {
			_, err = ib.pool.Add(ctx, msg, uint64(blk.Height))
			if err != nil {
				log.Info(err)
			}
		}
	}

	// Remove all messages in the new blocks from the pool, now mined.
	// Cid() can error, so collect all the CIDs up front.
	var removeCids []cid.Cid
	for _, blk := range newBlocks {
		for _, msg := range blk.Messages {
			cid, err := msg.Cid()
			if err != nil {
				return err
			}
			removeCids = append(removeCids, cid)
		}
	}
	for _, c := range removeCids {
		ib.pool.Remove(c)
	}

	// prune all messages that have been in the pool too long
	return timeoutMessages(ctx, ib.pool, ib.chain, newHead, ib.maxAgeTipsets)
}

// timeoutMessages removes all messages from the pool that arrived more than maxAgeTipsets tip sets ago.
// Note that we measure the timeout in the number of tip sets we have received rather than a fixed block
// height. This prevents us from prematurely timing messages that arrive during long chains of null blocks.
// Also when blocks fill, the rate of message processing will correspond more closely to rate of tip
// sets than to the expected block time over short timescales.
func timeoutMessages(ctx context.Context, pool *MessagePool, chains chain.BlockProvider, head types.TipSet, maxAgeTipsets uint) error {
	var err error

	lowestTipSet := head
	minimumHeight, err := lowestTipSet.Height()
	if err != nil {
		return err
	}

	// walk back MessageTimeout tip sets to arrive at the lowest viable block height
	for i := uint(0); minimumHeight > 0 && i < maxAgeTipsets; i++ {
		lowestTipSet, err = chain.GetParentTipSet(ctx, chains, lowestTipSet)
		if err != nil {
			return err
		}
		minimumHeight, err = lowestTipSet.Height()
		if err != nil {
			return err
		}
	}

	// remove all messages added before minimumHeight
	for _, cid := range pool.PendingBefore(minimumHeight) {
		pool.Remove(cid)
	}

	return nil
}
