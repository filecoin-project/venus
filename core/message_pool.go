package core

import (
	"sync"

	errors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/filecoin-project/go-filecoin/types"
)

// MessagePool contains all currently known messages.
// Messages are added to the pool, when
// - they are submitted locally
// - received through the network
//
// Messages are removed from the pool, when
// - they are included into the chain.
//
// Safe for concurrent access.
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
