package core

import (
	"context"

	logging "github.com/ipfs/go-log"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/types"
)

// OutboxMaxAgeRounds is the maximum age (in consensus rounds) to permit messages to stay in the outbound message queue.
// This should be a little longer than the message pool's timeout so that messages expire from mining
// pools a little before the sending node gives up on them.
const OutboxMaxAgeRounds = 10

var log = logging.Logger("core")

// QueuePolicy manages a message queue state in response to changes on the blockchain.
type QueuePolicy interface {
	// HandleNewHead updates a message queue in response to a new chain head. The new head may be based
	// directly on the previous head, or it may be based on a prior tipset (aka a re-org).
	// - `oldTips` is a list of tipsets that used to be on the main chain but are no longer.
	// - `newTips` is a list of tipsets that now form the head of the main chain.
	// Both lists are in descending height order, down to but not including the common ancestor tipset.
	HandleNewHead(ctx context.Context, target PolicyTarget, oldTips, newTips []types.TipSet) error
}

// PolicyTarget is outbound queue object on which the policy acts.
type PolicyTarget interface {
	RemoveNext(ctx context.Context, sender address.Address, expectedNonce uint64) (msg *types.SignedMessage, found bool, err error)
	ExpireBefore(ctx context.Context, stamp uint64) map[address.Address][]*types.SignedMessage
}

// DefaultQueuePolicy manages a target message queue state in response to changes on the blockchain.
// Messages are removed from the queue as soon as they appear in a block that's part of a heaviest chain.
// At this point, messages are highly likely to be valid and known to a large number of nodes,
// even if the block ends up as an abandoned fork.
// There is no special handling for re-orgs and messages do not revert to the queue if the block
// ends up childless (in contrast to the message pool).
type DefaultQueuePolicy struct {
	// Provides messages collections from cids.
	messageProvider MessageProvider
	// Maximum difference in message stamp from current block height before expiring an address's queue
	maxAgeRounds uint64
}

// NewMessageQueuePolicy returns a new policy which removes mined messages from the queue and expires
// messages older than `maxAgeTipsets` rounds.
func NewMessageQueuePolicy(messages MessageProvider, maxAge uint) *DefaultQueuePolicy {
	return &DefaultQueuePolicy{messages, uint64(maxAge)}
}

// HandleNewHead removes from the queue all messages that have now been mined in new blocks.
func (p *DefaultQueuePolicy) HandleNewHead(ctx context.Context, target PolicyTarget, oldTips, newTips []types.TipSet) error {
	// Rearrange the tipsets into increasing height order so messages are discovered in nonce order.
	chain.Reverse(newTips)
	for _, tipset := range newTips {
		for i := 0; i < tipset.Len(); i++ {
			msgs, err := p.messageProvider.LoadMessages(ctx, tipset.At(i).Messages)
			if err != nil {
				return err
			}
			for _, minedMsg := range msgs {
				removed, found, err := target.RemoveNext(ctx, minedMsg.From, uint64(minedMsg.Nonce))
				if err != nil {
					return err
				}
				if found && !minedMsg.Equals(removed) {
					log.Warningf("Queued message %v differs from mined message %v with same sender & nonce", removed, minedMsg)
				}
				// Else if not found, the message was not sent by this node, or has already been removed
				// from the queue (e.g. a blockchain re-org).
			}
		}
	}

	// Expire messages that have been in the queue for too long; they will probably never be mined.
	if len(newTips) > 0 {
		height, err := newTips[0].Height()
		if err != nil {
			return err
		}
		if height >= p.maxAgeRounds { // avoid uint subtraction overflow
			expired := target.ExpireBefore(ctx, (height - p.maxAgeRounds))
			for _, msg := range expired {
				log.Warningf("Outbound message %v expired un-mined after %d rounds", msg, p.maxAgeRounds)
			}
		}
	}
	return nil
}
