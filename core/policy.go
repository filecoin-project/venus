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

var log = logging.Logger("mqueue")

// QueuePolicy manages a message queue state in response to changes on the blockchain.
type QueuePolicy interface {
	// HandleNewHead updates a message queue in response to a new head tipset.
	HandleNewHead(ctx context.Context, target PolicyTarget, oldHead, newHead types.TipSet) error
}

// PolicyTarget is outbound queue object on which the policy acts.
type PolicyTarget interface {
	RemoveNext(sender address.Address, expectedNonce uint64) (msg *types.SignedMessage, found bool, err error)
	ExpireBefore(stamp uint64) map[address.Address][]*types.SignedMessage
}

// DefaultQueuePolicy manages a target message queue state in response to changes on the blockchain.
// Messages are removed from the queue as soon as they appear in a block that's part of a heaviest chain.
// At this point, messages are highly likely to be valid and known to a large number of nodes,
// even if the block ends up as an abandoned fork.
// There is no special handling for re-orgs and messages do not revert to the queue if the block
// ends up childless (in contrast to the message pool).
type DefaultQueuePolicy struct {
	// Provides blocks for chain traversal.
	store chain.BlockProvider
	// Maximum difference in message stamp from current block height before expiring an address's queue
	maxAgeRounds uint64
}

// NewMessageQueuePolicy returns a new policy which removes mined messages from the queue and expires
// messages older than `maxAgeRounds` rounds.
func NewMessageQueuePolicy(store chain.BlockProvider, maxAge uint64) *DefaultQueuePolicy {
	return &DefaultQueuePolicy{store, maxAge}
}

// HandleNewHead updates the policy target in response to a new head tipset.
func (p *DefaultQueuePolicy) HandleNewHead(ctx context.Context, target PolicyTarget, oldHead, newHead types.TipSet) error {
	_, newBlocks, err := CollectBlocksToCommonAncestor(ctx, p.store, oldHead, newHead)
	if err != nil {
		return err
	}

	// Remove from the queue all messages that have now been mined in new blocks.

	// Rearrange the blocks in increasing height order so messages are discovered in order.
	// Note: this is imperfect until CollectBlocksToCommonAncestor is updated to return blocks
	// at the same height in canonical (ticket) order.
	reverse(newBlocks)
	for _, block := range newBlocks {
		for _, minedMsg := range block.Messages {
			removed, found, err := target.RemoveNext(minedMsg.From, uint64(minedMsg.Nonce))
			if err != nil {
				return err
			}
			if found && !minedMsg.Equals(removed) {
				log.Errorf("Queued message %v differs from mined message %v with same sender & nonce", removed, minedMsg)
			}
			// Else if not found, the message was not sent by this node, or has already been removed
			// from the queue (e.g. a blockchain re-org).
		}
	}

	// Expire messages that have been in the queue for too long; they will probably never be mined.
	height, err := newHead.Height()
	if err != nil {
		return err
	}
	if height >= p.maxAgeRounds { // avoid uint subtraction overflow
		expired := target.ExpireBefore(height - p.maxAgeRounds)
		for _, msg := range expired {
			log.Errorf("Outbound message %v expired un-mined after %d rounds", msg, p.maxAgeRounds)
		}
	}
	return nil
}

func reverse(list []*types.Block) {
	// https://github.com/golang/go/wiki/SliceTricks#reversing
	for i := len(list)/2 - 1; i >= 0; i-- {
		opp := len(list) - 1 - i
		list[i], list[opp] = list[opp], list[i]
	}
}
