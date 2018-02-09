package core

import (
	"sync"

	errors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

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
func (pool *MessagePool) Add(msg *types.Message) error {
	pool.lk.Lock()
	defer pool.lk.Unlock()

	c, err := msg.Cid()
	if err != nil {
		return errors.Wrap(err, "failed to create CID")
	}

	pool.pending[c.KeyString()] = msg
	return nil
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
