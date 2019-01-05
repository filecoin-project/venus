package core

import (
	"context"
	"sort"
	"sync"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/types"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmRXf2uUSdGSunRJsM9wXSUNVwLUGCY3So5fAs7h2CBJVf/go-hamt-ipld"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
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

	pending map[cid.Cid]*types.SignedMessage // all pending messages
}

// Add adds a message to the pool.
func (pool *MessagePool) Add(msg *types.SignedMessage) (cid.Cid, error) {
	pool.lk.Lock()
	defer pool.lk.Unlock()

	c, err := msg.Cid()
	if err != nil {
		return cid.Undef, errors.Wrap(err, "failed to create CID")
	}

	// Reject messages with invalid signatires
	if !msg.VerifySignature() {
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
		out = append(out, msg)
	}

	return out
}

// Remove removes the message by CID from the pending pool.
func (pool *MessagePool) Remove(c cid.Cid) {
	pool.lk.Lock()
	defer pool.lk.Unlock()

	delete(pool.pending, c)
}

// NewMessagePool constructs a new MessagePool.
func NewMessagePool() *MessagePool {
	return &MessagePool{
		pending: make(map[cid.Cid]*types.SignedMessage),
	}
}

// getParentTips returns the parent tipset of the provided tipset
// TODO msgPool should have access to a chain store that can just look this up...
func getParentTipSet(ctx context.Context, store *hamt.CborIpldStore, ts consensus.TipSet) (consensus.TipSet, error) {
	newTipSet := consensus.TipSet{}
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
func collectChainsMessagesToHeight(ctx context.Context, store *hamt.CborIpldStore, curTipSet consensus.TipSet, height uint64) ([]*types.SignedMessage, consensus.TipSet, error) {
	var msgs []*types.SignedMessage
	h, err := curTipSet.Height()
	if err != nil {
		return nil, nil, err
	}
	for h > height {
		for _, blk := range curTipSet {
			msgs = append(msgs, blk.Messages...)
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
				return []*types.SignedMessage{}, consensus.TipSet{}, err
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
func UpdateMessagePool(ctx context.Context, pool *MessagePool, store *hamt.CborIpldStore, old, new consensus.TipSet) error {
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
				addToPool = append(addToPool, blk.Messages...)
			}
		}
		for _, blk := range new {
			removeFromPool = append(removeFromPool, blk.Messages...)
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
		_, err := pool.Add(m)
		if err != nil {
			return err
		}
	}
	// m.Cid() can error, so collect all the Cids before
	removeCids := make([]cid.Cid, len(removeFromPool))
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

// OrderMessagesByNonce returns the pending messages in the
// pool ordered such that all messages with the same msg.From
// occur in Nonce order in the slice.
// TODO can be smarter here by skipping messages with gaps; see
//      ethereum's abstraction for example
// TODO order by time of receipt
func OrderMessagesByNonce(messages []*types.SignedMessage) []*types.SignedMessage {
	// TODO this could all be more efficient.
	byAddress := make(map[address.Address][]*types.SignedMessage)
	for _, m := range messages {
		byAddress[m.From] = append(byAddress[m.From], m)
	}
	messages = messages[:0]
	for _, msgs := range byAddress {
		sort.Slice(msgs, func(i, j int) bool { return msgs[i].Nonce < msgs[j].Nonce })
		messages = append(messages, msgs...)
	}
	return messages
}

// LargestNonce returns the largest nonce used by a message from address in the pool.
// If no messages from address are found, found will be false.
func LargestNonce(pool *MessagePool, address address.Address) (largest uint64, found bool) {
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
