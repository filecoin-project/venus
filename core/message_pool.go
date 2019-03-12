package core

import (
	"context"
	"sync"

	"gx/ipfs/QmNf3wujpV2Y7Lnj2hy2UrmuX8bhMDStRHbnSLh7Ypf36h/go-hamt-ipld"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/address"
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

// getParentTips returns the parent tipset of the provided tipset
// TODO msgPool should have access to a chain store that can just look this up...
func getParentTipSet(ctx context.Context, store *hamt.CborIpldStore, ts types.TipSet) (types.TipSet, error) {
	newTipSet := types.TipSet{}
	parents, err := ts.Parents()
	if err != nil {
		return nil, err
	}
	for it := parents.Iter(); !it.Complete() && ctx.Err() == nil; it.Next() {
		var newBlk types.Block
		if err := store.Get(ctx, it.Value(), &newBlk); err != nil {
			return nil, err
		}
		if err := newTipSet.AddBlock(&newBlk); err != nil {
			return nil, err
		}
	}
	return newTipSet, nil
}

// collectChainsMessagesToHeight is a helper that collects all the messages
// from block `b` down the chain to but not including its ancestor of
// height `height`.  This function returns the messages collected along with
// the tipset at the final height.
// TODO ripe for optimizing away lots of allocations
func collectChainsMessagesToHeight(ctx context.Context, store *hamt.CborIpldStore, curTipSet types.TipSet, height uint64) ([]*timedmessage, types.TipSet, error) {
	var msgs []*timedmessage
	h, err := curTipSet.Height()
	if err != nil {
		return nil, nil, err
	}
	for h > height {
		for _, blk := range curTipSet {
			for _, msg := range blk.Messages {
				msgs = append(msgs, &timedmessage{message: msg, addedAt: uint64(blk.Height)})
			}
		}
		parents, err := curTipSet.Parents()
		if err != nil {
			return nil, nil, err
		}
		switch parents.Len() {
		case 0:
			return msgs, curTipSet, nil
		default:
			nextTipSet, err := getParentTipSet(ctx, store, curTipSet)
			if err != nil {
				return []*timedmessage{}, types.TipSet{}, err
			}
			curTipSet = nextTipSet
			h, err = curTipSet.Height()
			if err != nil {
				return nil, nil, err
			}
		}
	}
	return msgs, curTipSet, nil
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
func (pool *MessagePool) UpdateMessagePool(ctx context.Context, store *hamt.CborIpldStore, old, new types.TipSet) error {
	// Strategy: walk head-of-chain pointers old and new back until they are at the same
	// height, then walk back in lockstep to find the common ancesetor.

	// If old is higher/longer than new, collect all the messages
	// from old's chain down to the height of new.
	newHeight, err := new.Height()
	if err != nil {
		return err
	}
	addToPool, old, err := collectChainsMessagesToHeight(ctx, store, old, newHeight)
	if err != nil {
		return err
	}
	// If new is higher/longer than old, collect all the messages
	// from new's chain down to the height of old.
	oldHeight, err := old.Height()
	if err != nil {
		return err
	}
	removeFromPool, new, err := collectChainsMessagesToHeight(ctx, store, new, oldHeight)
	if err != nil {
		return err
	}
	// Old and new are now at the same height. Keep walking them down a
	// tipset at a time in lockstep until they are pointing to the same
	// tipset, the common ancestor. Collect their messages to add/remove
	// along the way.
	//
	// TODO probably should limit depth here.
	for !old.Equals(new) {
		for _, blk := range old {
			// skip genesis block
			if blk.Height > 0 {
				for _, msg := range blk.Messages {
					addToPool = append(addToPool, &timedmessage{message: msg, addedAt: uint64(blk.Height)})
				}
			}
		}
		for _, blk := range new {
			for _, msg := range blk.Messages {
				removeFromPool = append(removeFromPool, &timedmessage{message: msg, addedAt: uint64(blk.Height)})
			}
		}
		oldParents, err := old.Parents()
		if err != nil {
			return err
		}
		newParents, err := new.Parents()
		if err != nil {
			return err
		}
		if oldParents.Empty() || newParents.Empty() {
			break
		}
		old, err = getParentTipSet(ctx, store, old)
		if err != nil {
			return err
		}
		new, err = getParentTipSet(ctx, store, new)
		if err != nil {
			return err
		}
	}

	// Now actually update the pool.
	for _, m := range addToPool {
		_, err := pool.addTimedMessage(m)
		if err != nil {
			return err
		}
	}
	// m.Cid() can error, so collect all the Cids before
	removeCids := make([]cid.Cid, len(removeFromPool))
	for i, m := range removeFromPool {
		cid, err := m.message.Cid()
		if err != nil {
			return err
		}
		removeCids[i] = cid
	}
	for _, cid := range removeCids {
		pool.Remove(cid)
	}

	// prune all messages that have been in the pool too long
	return pool.timeoutMessages(ctx, store, new)
}

// timeoutMessages removes all messages from the pool that arrived more than MessageTimeout tip sets ago.
// Note that we measure the timeout in the number of tip sets we have received rather than a fixed block
// height. This prevents us from prematurely timing messages that arrive during long chains of null blocks.
// Also when blocks fill, the rate of message processing will correspond more closely to rate of tip
// sets than to the expected block time over short timescales.
func (pool *MessagePool) timeoutMessages(ctx context.Context, store *hamt.CborIpldStore, head types.TipSet) error {
	var err error

	lowestTipSet := head
	minimumHeight, err := lowestTipSet.Height()
	if err != nil {
		return err
	}

	// walk back MessageTimeout tip sets to arrive at the lowest viable block height
	for i := 0; minimumHeight > 0 && i < MessageTimeOut; i++ {
		lowestTipSet, err = getParentTipSet(ctx, store, lowestTipSet)
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
