package core

import (
	"context"
	"sync"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/types"
)

// MessageTimeOut is the number of tipsets we should receive before timing out messages
const MessageTimeOut = 6

type timedmessage struct {
	message *types.SignedMessage
	addedAt uint64
}

// BlockTimer defines a interface to a struct that can give the current block height.
type BlockTimer interface {
	BlockHeight() (uint64, error)
}

// MessagePool keeps an unordered, de-duplicated set of Messages and supports removal by CID.
// By 'de-duplicated' we mean that insertion of a message by cid that already
// exists is a nop. We use a MessagePool to store all messages received by this node
// via network or directly created via user command that have yet to be included
// in a block. Messages are removed as they are processed.
//
// MessagePool is safe for concurrent access.
type MessagePool struct {
	lk sync.RWMutex

	timer   BlockTimer
	pending map[cid.Cid]*timedmessage // all pending messages
}

// Add adds a message to the pool.
func (pool *MessagePool) Add(msg *types.SignedMessage) (cid.Cid, error) {
	blockTime, err := pool.timer.BlockHeight()
	if err != nil {
		return cid.Undef, err
	}

	return pool.addTimedMessage(&timedmessage{message: msg, addedAt: blockTime})
}

func (pool *MessagePool) addTimedMessage(msg *timedmessage) (cid.Cid, error) {
	pool.lk.Lock()
	defer pool.lk.Unlock()

	c, err := msg.message.Cid()
	if err != nil {
		return cid.Undef, errors.Wrap(err, "failed to create CID")
	}

	// Reject messages with invalid signatires
	if !msg.message.VerifySignature() {
		return cid.Undef, errors.Errorf("failed to add message %s to pool: sig invalid", c.String())
	}

	pool.pending[c] = msg
	return c, nil

}

// Pending returns all pending messages.
func (pool *MessagePool) Pending() []*types.SignedMessage {
	pool.lk.Lock()
	defer pool.lk.Unlock()
	out := make([]*types.SignedMessage, 0, len(pool.pending))
	for _, msg := range pool.pending {
		out = append(out, msg.message)
	}

	return out
}

// Get retrieves a message from the pool by CID.
func (pool *MessagePool) Get(c cid.Cid) (*types.SignedMessage, bool) {
	pool.lk.Lock()
	defer pool.lk.Unlock()
	value, ok := pool.pending[c]
	if !ok {
		return nil, ok
	} else if value == nil {
		panic("Found nil message for CID " + c.String())
	}
	return value.message, ok
}

// Remove removes the message by CID from the pending pool.
func (pool *MessagePool) Remove(c cid.Cid) {
	pool.lk.Lock()
	defer pool.lk.Unlock()

	delete(pool.pending, c)
}

// NewMessagePool constructs a new MessagePool.
func NewMessagePool(timer BlockTimer) *MessagePool {
	return &MessagePool{
		timer:   timer,
		pending: make(map[cid.Cid]*timedmessage),
	}
}

// UpdateMessagePool brings the message pool into the correct state after
// we accept a new block. It removes messages from the pool that are
// found in the newly adopted chain and adds back those from the removed
// chain (if any) that do not appear in the new chain. We think
// that the right model for keeping the message pool up to date is
// to think about it like a garbage collector.
//
// TODO there is considerable functionality missing here: don't add
//      messages that have expired, respect nonce, do this efficiently,
//      etc.
func (pool *MessagePool) UpdateMessagePool(ctx context.Context, store chain.BlockProvider, oldHead, newHead types.TipSet) error {
	oldBlocks, newBlocks, err := CollectBlocksToCommonAncestor(ctx, store, oldHead, newHead)
	if err != nil {
		return err
	}

	// Add all message from the old blocks to the message pool, so they can be mined again.
	for _, blk := range oldBlocks {
		for _, msg := range blk.Messages {
			_, err = pool.addTimedMessage(&timedmessage{message: msg, addedAt: uint64(blk.Height)})
			if err != nil {
				return err
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
		pool.Remove(c)
	}

	// prune all messages that have been in the pool too long
	return pool.timeoutMessages(ctx, store, newHead)
}

// timeoutMessages removes all messages from the pool that arrived more than MessageTimeout tip sets ago.
// Note that we measure the timeout in the number of tip sets we have received rather than a fixed block
// height. This prevents us from prematurely timing messages that arrive during long chains of null blocks.
// Also when blocks fill, the rate of message processing will correspond more closely to rate of tip
// sets than to the expected block time over short timescales.
func (pool *MessagePool) timeoutMessages(ctx context.Context, store chain.BlockProvider, head types.TipSet) error {
	var err error

	lowestTipSet := head
	minimumHeight, err := lowestTipSet.Height()
	if err != nil {
		return err
	}

	// walk back MessageTimeout tip sets to arrive at the lowest viable block height
	for i := 0; minimumHeight > 0 && i < MessageTimeOut; i++ {
		lowestTipSet, err = chain.GetParentTipSet(ctx, store, lowestTipSet)
		if err != nil {
			return err
		}
		minimumHeight, err = lowestTipSet.Height()
		if err != nil {
			return err
		}
	}

	// remove all messages added before minimumHeight
	for cid, msg := range pool.pending {
		if msg.addedAt < minimumHeight {
			pool.Remove(cid)
		}
	}

	return nil
}

// LargestNonce returns the largest nonce used by a message from address in the pool.
// If no messages from address are found, found will be false.
func (pool *MessagePool) LargestNonce(address address.Address) (largest uint64, found bool) {
	for _, m := range pool.Pending() {
		if m.From == address {
			found = true
			if uint64(m.Nonce) > largest {
				largest = uint64(m.Nonce)
			}
		}
	}
	return
}
