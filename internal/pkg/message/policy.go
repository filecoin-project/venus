package message

import (
	"context"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	logging "github.com/ipfs/go-log/v2"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/chain"
	"github.com/filecoin-project/venus/internal/pkg/types"
)

// OutboxMaxAgeRounds is the maximum age (in consensus rounds) to permit messages to stay in the outbound message queue.
// This should be a little longer than the message pool's timeout so that messages expire from mining
// pools a little before the sending node gives up on them.
const OutboxMaxAgeRounds = 10

var log = logging.Logger("message")

// QueuePolicy manages a message queue state in response to changes on the blockchain.
type QueuePolicy interface {
	MessagesForTipset(context.Context, *block.TipSet) ([]types.ChainMsg, error)

	// HandleNewHead updates a message queue in response to a new chain head. The new head may be based
	// directly on the previous head, or it may be based on a prior tipset (aka a re-org).
	// - `oldTips` is a list of tipsets that used to be on the main chain but are no longer.
	// - `newTips` is a list of tipsets that now form the head of the main chain.
	// Both lists are in descending height order, down to but not including the common ancestor tipset.
	HandleNewHead(ctx context.Context, target PolicyTarget, oldTips, newTips []*block.TipSet) error
}

// PolicyTarget is outbound queue object on which the policy acts.
type PolicyTarget interface {
	RemoveNext(ctx context.Context, sender address.Address, expectedNonce uint64) (msg *types.SignedMessage, found bool, err error)
	Requeue(ctx context.Context, msg *types.SignedMessage, stamp uint64) error
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
	messageProvider messageProvider
	// Maximum difference in message stamp from current block height before expiring an address's queue
	maxAgeRounds uint64
}

// NewMessageQueuePolicy returns a new policy which removes mined messages from the queue and expires
// messages older than `maxAgeTipsets` rounds.
func NewMessageQueuePolicy(messages messageProvider, maxAge uint) *DefaultQueuePolicy {
	return &DefaultQueuePolicy{messages, uint64(maxAge)}
}

// HandleNewHead removes from the queue all messages that have now been mined in new blocks.
func (p *DefaultQueuePolicy) HandleNewHead(ctx context.Context, target PolicyTarget, oldTips, newTips []*block.TipSet) error {
	chainHeight, err := reorgHeight(oldTips, newTips)
	if err != nil {
		return err
	}

	// Remove all messages in the new chain from the queue since they have been mined into blocks.
	// Rearrange the tipsets into ascending height order so messages are discovered in nonce order.
	chain.Reverse(newTips)
	for _, tipset := range newTips {
		for i := 0; i < tipset.Len(); i++ {
			secpMsgs, _, err := p.messageProvider.LoadMetaMessages(ctx, tipset.At(i).Messages.Cid)
			if err != nil {
				return err
			}
			for _, minedMsg := range secpMsgs {
				removed, found, err := target.RemoveNext(ctx, minedMsg.Message.From, minedMsg.Message.CallSeqNum)
				if err != nil {
					return err
				}
				if found && !minedMsg.Equals(removed) {
					log.Warnf("Queued message %v differs from mined message %v with same sender & nonce", removed, minedMsg)
				}
				// Else if not found, the message was not sent by this node, or has already been removed
				// from the queue (e.g. a blockchain re-org).
			}
		}
	}

	// Return messages from the old chain back to the queue. This is necessary so that the next nonce
	// implied by the queue+state matches that of the message pool (which will also have the un-mined
	// message re-instated).
	// Note that this will include messages that were never sent by this node since the queue doesn't
	// keep track of "allowed" senders. However, messages from other addresses will expire
	// harmlessly.
	// See discussion in https://github.com/filecoin-project/venus/issues/3052
	// Traverse these in descending height order.
	for _, tipset := range oldTips {
		for i := 0; i < tipset.Len(); i++ {
			secpMsgs, _, err := p.messageProvider.LoadMetaMessages(ctx, tipset.At(i).Messages.Cid)
			if err != nil {
				return err
			}
			for _, restoredMsg := range secpMsgs {
				err := target.Requeue(ctx, restoredMsg, uint64(chainHeight))
				if err != nil {
					return err
				}
			}
		}
	}

	// Expire messages that have been in the queue for too long; they will probably never be mined.
	if uint64(chainHeight) >= p.maxAgeRounds { // avoid uint subtraction overflow
		expired := target.ExpireBefore(ctx, uint64(chainHeight)-p.maxAgeRounds)
		for _, msg := range expired {
			log.Warnf("Outbound message %v expired un-mined after %d rounds", msg, p.maxAgeRounds)
		}
	}
	return nil
}

type BlockMessages struct {
	Miner         address.Address
	BlsMessages   []types.ChainMsg
	SecpkMessages []types.ChainMsg
	WinCount      int64
}

func (p *DefaultQueuePolicy) blockMsgsForTipset(ctx context.Context, ts *block.TipSet) ([]BlockMessages, error) {
	applied := make(map[address.Address]uint64)

	selectMsg := func(m *types.UnsignedMessage) (bool, error) {
		// The first match for a sender is guaranteed to have correct nonce -- the block isn't valid otherwise
		if _, ok := applied[m.From]; !ok {
			applied[m.From] = m.CallSeqNum
		}

		if applied[m.From] != m.CallSeqNum {
			return false, nil
		}

		applied[m.From]++

		return true, nil
	}

	var out []BlockMessages
	for _, b := range ts.Blocks() {
		secpMsgs, blsMsgs, err := p.messageProvider.LoadMetaMessages(ctx, b.Messages.Cid) // cs.MessagesForBlock(b)
		if err != nil {
			return nil, xerrors.Errorf("failed to get messages for block: %w", err)
		}

		bm := BlockMessages{
			Miner:         b.Miner,
			BlsMessages:   make([]types.ChainMsg, 0, len(blsMsgs)),
			SecpkMessages: make([]types.ChainMsg, 0, len(secpMsgs)),
			WinCount:      b.ElectionProof.WinCount,
		}

		for _, bmsg := range blsMsgs {
			b, err := selectMsg(bmsg.VMMessage())
			if err != nil {
				return nil, xerrors.Errorf("failed to decide whether to select message for block: %w", err)
			}

			if b {
				bm.BlsMessages = append(bm.BlsMessages, bmsg)
			}
		}

		for _, smsg := range secpMsgs {
			b, err := selectMsg(smsg.VMMessage())
			if err != nil {
				return nil, xerrors.Errorf("failed to decide whether to select message for block: %w", err)
			}

			if b {
				bm.SecpkMessages = append(bm.SecpkMessages, smsg)
			}
		}

		out = append(out, bm)
	}

	return out, nil
}

func (p *DefaultQueuePolicy) MessagesForTipset(ctx context.Context, ts *block.TipSet) ([]types.ChainMsg, error) {
	bmsgs, err := p.blockMsgsForTipset(ctx, ts)
	if err != nil {
		return nil, err
	}

	var out []types.ChainMsg
	for _, bm := range bmsgs {
		for _, blsm := range bm.BlsMessages {
			out = append(out, blsm)
		}

		for _, secm := range bm.SecpkMessages {
			out = append(out, secm)
		}
	}

	return out, nil
}

// reorgHeight returns height of the new chain given only the tipset diff which may be empty
func reorgHeight(oldTips, newTips []*block.TipSet) (abi.ChainEpoch, error) {
	if len(newTips) > 0 {
		return newTips[0].Height()
	} else if len(oldTips) > 0 { // A pure rewind is unlikely in practice.
		return oldTips[0].Height()
	}
	// this is a noop reorg. Chain height shouldn't matter.
	return 0, nil
}
