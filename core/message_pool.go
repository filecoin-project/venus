package core

import (
	"context"
	"sync"

	errors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	hamt "gx/ipfs/QmdtiofXbibTe6Day9ii5zjBZpSRm8vhfoerrNuY3sAQ7e/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/types"
)

// MessagePool keeps an unordered, de-duplicated set of Messages and supports removal by CID.
// By 'de-duplicated' we mean that insertion of a message by cid that already
// exists is a nop. We use a MessagePool to store all messages received by this node
// via network or directly created via user command that have yet to be included
// in a block. Messages are removed as they are processed.
//
// MessagePool is safe for concurrent access.
type MessagePool struct {
	lk sync.RWMutex

	pending map[string]*types.Message // all pending messages
}

// Add adds a message to the pool.
func (pool *MessagePool) Add(msg *types.Message) (*cid.Cid, error) {
	pool.lk.Lock()
	defer pool.lk.Unlock()

	c, err := msg.Cid()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create CID")
	}

	pool.pending[c.KeyString()] = msg
	return c, nil
}

// Pending returns all pending messages.
func (pool *MessagePool) Pending() []*types.Message {
	out := make([]*types.Message, 0, len(pool.pending))
	for _, msg := range pool.pending {
		out = append(out, msg)
	}

	return out
}

// Remove removes the message by CID from the pending pool.
func (pool *MessagePool) Remove(c *cid.Cid) {
	delete(pool.pending, c.KeyString())
}

// NewMessagePool constructs a new MessagePool.
func NewMessagePool() *MessagePool {
	return &MessagePool{
		pending: make(map[string]*types.Message),
	}
}

// collectChainsMessagesToHeight is a helper that collects all the messages
// from block `b` down the chain to but not including its ancestor of
// height `height`. It updates and re-uses the space pointed to by curBlock.
func collectChainsMessagesToHeight(store *hamt.CborIpldStore, curBlock *types.Block, height uint64) ([]*types.Message, error) {
	msgs := []*types.Message{}
	for curBlock.Height > height {
		msgs = append(msgs, curBlock.Messages...)
		if curBlock.Parent == nil {
			return msgs, nil
		}
		if err := store.Get(context.TODO(), curBlock.Parent, curBlock); err != nil {
			return nil, err
		}
	}
	return msgs, nil
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
func UpdateMessagePool(ctx context.Context, pool *MessagePool, store *hamt.CborIpldStore, oldHead, newHead *types.Block) error {
	// Strategy: walk pointers oldHead and newHead back until they are at the same
	// height, then walk back in lockstep to find the common ancesetor.

	// Use new storage for oldHead and newHead to avoid aliasing with
	// the nodes pointed to by the arguments when loading blocks.
	var old, new types.Block
	if err := store.Get(ctx, oldHead.Cid(), &old); err != nil {
		return err
	}
	if err := store.Get(ctx, newHead.Cid(), &new); err != nil {
		return err
	}

	// If old is higher/longer than new, collect all the messages
	// from old's chain down to the height of new.
	addToPool, err := collectChainsMessagesToHeight(store, &old, new.Height)
	if err != nil {
		return err
	}
	// If new is higher/longer than old, collect all the messages
	// from its chain down to the height of new.
	removeFromPool, err := collectChainsMessagesToHeight(store, &new, old.Height)
	if err != nil {
		return err
	}

	// Old and new are now at the same height. Keep walking them down a
	// block at a time in lockstep until they are pointing to the same
	// node, the common ancestor. Collect their messages to add/remove
	// along the way.
	//
	// TODO probably should limit depth here.
	for !old.Cid().Equals(new.Cid()) {
		addToPool = append(addToPool, old.Messages...)
		removeFromPool = append(removeFromPool, new.Messages...)
		if old.Parent == nil || new.Parent == nil {
			break
		}
		if err := store.Get(ctx, old.Parent, &old); err != nil {
			return err
		}
		if err := store.Get(ctx, new.Parent, &new); err != nil {
			return err
		}
	}

	// Now actually update the pool.
	for _, m := range addToPool {
		_, err := pool.Add(m)
		if err != nil {
			return err
		}
	}
	// m.Cid() can error, so collect all the Cids before
	removeCids := make([]*cid.Cid, len(removeFromPool))
	for i, m := range removeFromPool {
		cid, err := m.Cid()
		if err != nil {
			return err
		}
		removeCids[i] = cid
	}
	for _, cid := range removeCids {
		pool.Remove(cid)
	}

	return nil
}
