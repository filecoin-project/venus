package message

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// InboxMaxAgeTipsets is maximum age (in non-empty tipsets) to permit messages to stay in the pool after reception.
// It should be a little shorter than the outbox max age so that messages expire from mining
// pools a little before the sender gives up on them.
const InboxMaxAgeTipsets = 6

// Inbox maintains a pool of received messages.
type Inbox struct {
	// The pool storing received messages.
	pool *Pool
	// Maximum age of a pool message.
	maxAgeTipsets uint

	// Provides tipsets for chain traversal.
	chain           chainProvider
	messageProvider messageProvider
}

// messageProvider provides message collections given their cid.
type messageProvider interface {
	LoadMessages(context.Context, types.TxMeta) ([]*types.SignedMessage, []*types.UnsignedMessage, error)
}

// NewInbox constructs a new inbox.
func NewInbox(pool *Pool, maxAgeRounds uint, chain chainProvider, messages messageProvider) *Inbox {
	return &Inbox{
		pool:            pool,
		maxAgeTipsets:   maxAgeRounds,
		chain:           chain,
		messageProvider: messages,
	}
}

// Add adds a message to the pool, tagged with the current block height.
// An error probably means the message failed to validate,
// but it could indicate a more serious problem with the system.
func (ib *Inbox) Add(ctx context.Context, msg *types.SignedMessage) (cid.Cid, error) {
	head, err := ib.chain.GetTipSet(ib.chain.GetHead())
	if err != nil {
		return cid.Undef, err
	}
	blockTime, err := head.Height()
	if err != nil {
		return cid.Undef, err
	}

	return ib.pool.Add(ctx, msg, blockTime)
}

// Pool returns the inbox's message pool.
func (ib *Inbox) Pool() *Pool {
	return ib.pool
}

// HandleNewHead updates the message pool in response to a new head tipset.
// This removes messages from the pool that are found in the newly adopted chain and adds back
// those from the removed chain (if any) that do not appear in the new chain.
// The `oldChain` and `newChain` lists are expected in descending height order, and each may be empty.
func (ib *Inbox) HandleNewHead(ctx context.Context, oldChain, newChain []block.TipSet) error {
	chainHeight, err := reorgHeight(oldChain, newChain)
	if err != nil {
		return err
	}

	// Add all message from the old tipsets to the message pool, so they can be mined again.
	for _, tipset := range oldChain {
		for i := 0; i < tipset.Len(); i++ {
			block := tipset.At(i)
			secpMsgs, _, err := ib.messageProvider.LoadMessages(ctx, block.Messages)
			if err != nil {
				return err
			}
			for _, msg := range secpMsgs {
				_, err = ib.pool.Add(ctx, msg, chainHeight)
				if err != nil {
					// Messages from the removed chain are frequently invalidated, e.g. because that
					// same message is already mined on the new chain.
					log.Debug(err)
				}
			}
		}
	}

	// Remove all messages in the new tipsets from the pool, now mined.
	// Cid() can error, so collect all the CIDs up front.
	var removeCids []cid.Cid
	for _, tipset := range newChain {
		for i := 0; i < tipset.Len(); i++ {
			secpMsgs, _, err := ib.messageProvider.LoadMessages(ctx, tipset.At(i).Messages)
			if err != nil {
				return err
			}
			for _, msg := range secpMsgs {
				cid, err := msg.Cid()
				if err != nil {
					return err
				}
				removeCids = append(removeCids, cid)
			}
		}
	}
	for _, c := range removeCids {
		ib.pool.Remove(c)
	}

	// prune all messages that have been in the pool too long
	if len(newChain) > 0 {
		return timeoutMessages(ctx, ib.pool, ib.chain, newChain[0], ib.maxAgeTipsets)
	}
	return nil
}

// timeoutMessages removes all messages from the pool that arrived more than maxAgeTipsets tip sets ago.
// Note that we measure the timeout in the number of tip sets we have received rather than a fixed block
// height. This prevents us from prematurely timing messages that arrive during long chains of null blocks.
// Also when blocks fill, the rate of message processing will correspond more closely to rate of tip
// sets than to the expected block time over short timescales.
func timeoutMessages(ctx context.Context, pool *Pool, chains chain.TipSetProvider, head block.TipSet, maxAgeTipsets uint) error {
	var err error

	var minimumHeight uint64
	itr := chain.IterAncestors(ctx, chains, head)

	// Walk back maxAgeTipsets+1 tipsets to determine lowest block height to prune.
	for i := uint(0); err == nil && i <= maxAgeTipsets && !itr.Complete(); i++ {
		minimumHeight, err = itr.Value().Height()
		if err == nil {
			err = itr.Next()
		}
	}
	if err != nil {
		return err
	}

	// remove all messages added before minimumHeight
	for _, cid := range pool.PendingBefore(minimumHeight) {
		pool.Remove(cid)
	}

	return nil
}
