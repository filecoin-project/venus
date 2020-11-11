package message

import (
	"context"
	"sync"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/internal/pkg/config"
	"github.com/filecoin-project/venus/internal/pkg/metrics"
	"github.com/filecoin-project/venus/internal/pkg/types"
)

var mpSize = metrics.NewInt64Gauge("message_pool_size", "The size of the message pool")

// PoolValidator defines a validator that ensures a message can go through the pool.
type PoolValidator interface {
	ValidateSignedMessageSyntax(ctx context.Context, msg *types.SignedMessage) error
}

// Pool keeps an unordered, de-duplicated set of Messages and supports removal by CID.
// By 'de-duplicated' we mean that insertion of a message by cid that already
// exists is a nop. We use a Pool to store all messages received by this node
// via network or directly created via user command that have yet to be included
// in a block. Messages are removed as they are processed.
//
// Pool is safe for concurrent access.
type Pool struct {
	lk sync.RWMutex

	cfg           *config.MessagePoolConfig
	validator     PoolValidator
	pending       map[cid.Cid]*timedmessage // all pending messages
	addressNonces map[addressNonce]bool     // set of address nonce pairs used to efficiently validate duplicate nonces
}

type timedmessage struct {
	message *types.SignedMessage
	addedAt abi.ChainEpoch
}

type addressNonce struct {
	addr  address.Address
	nonce uint64
}

func newAddressNonce(msg *types.SignedMessage) addressNonce {
	return addressNonce{addr: msg.Message.From, nonce: msg.Message.CallSeqNum}
}

// NewPool constructs a new Pool.
func NewPool(cfg *config.MessagePoolConfig, validator PoolValidator) *Pool {
	return &Pool{
		cfg:           cfg,
		validator:     validator,
		pending:       make(map[cid.Cid]*timedmessage),
		addressNonces: make(map[addressNonce]bool),
	}
}

// Add adds a message to the pool, tagged with the block height at which it was received.
// Does nothing if the message is already in the pool.
func (pool *Pool) Add(ctx context.Context, msg *types.SignedMessage, height abi.ChainEpoch) (cid.Cid, error) {
	pool.lk.Lock()
	defer pool.lk.Unlock()

	c, err := msg.Cid()
	if err != nil {
		return cid.Undef, errors.Wrap(err, "failed to create CID")
	}

	// ignore message prior to validation if it is already in pool
	_, found := pool.pending[c]
	if found {
		return c, nil
	}

	if err = pool.validateMessage(ctx, msg); err != nil {
		return cid.Undef, errors.Wrap(err, "validation error adding message to pool")
	}

	pool.pending[c] = &timedmessage{message: msg, addedAt: height}
	pool.addressNonces[newAddressNonce(msg)] = true
	mpSize.Set(ctx, int64(len(pool.pending)))
	return c, nil
}

// Pending returns all pending messages.
func (pool *Pool) Pending() []*types.SignedMessage {
	pool.lk.Lock()
	defer pool.lk.Unlock()

	out := make([]*types.SignedMessage, 0, len(pool.pending))
	for _, msg := range pool.pending {
		out = append(out, msg.message)
	}

	return out
}

// Get retrieves a message from the pool by CID.
func (pool *Pool) Get(c cid.Cid) (*types.SignedMessage, bool) {
	pool.lk.RLock()
	defer pool.lk.RUnlock()
	value, ok := pool.pending[c]
	if !ok {
		return nil, ok
	} else if value == nil {
		panic("Found nil message for CID " + c.String())
	}
	return value.message, ok
}

// Remove removes the message by CID from the pending pool.
func (pool *Pool) Remove(c cid.Cid) {
	pool.lk.Lock()
	defer pool.lk.Unlock()
	msg, ok := pool.pending[c]
	if ok {
		delete(pool.addressNonces, newAddressNonce(msg.message))
		delete(pool.pending, c)
	}

	mpSize.Set(context.TODO(), int64(len(pool.pending)))
}

// LargestNonce returns the largest nonce used by a message from address in the pool.
// If no messages from address are found, found will be false.
func (pool *Pool) LargestNonce(address address.Address) (largest uint64, found bool) {
	for _, m := range pool.Pending() {
		if m.Message.From == address {
			found = true
			if m.Message.CallSeqNum > largest {
				largest = m.Message.CallSeqNum
			}
		}
	}
	return
}

// PendingBefore returns the CIDs of messages added with height less than `minimumHeight`.
func (pool *Pool) PendingBefore(minimumHeight abi.ChainEpoch) []cid.Cid {
	pool.lk.RLock()
	defer pool.lk.RUnlock()

	var cids []cid.Cid
	for c, msg := range pool.pending {
		if msg.addedAt < minimumHeight {
			cids = append(cids, c)
		}
	}
	return cids
}

// validateMessage validates that too many messages aren't added to the pool and the ones that are
// have a high probability of making it through processing.
func (pool *Pool) validateMessage(ctx context.Context, message *types.SignedMessage) error {
	if uint(len(pool.pending)) >= pool.cfg.MaxPoolSize {
		return errors.Errorf("message pool is full (%d messages)", pool.cfg.MaxPoolSize)
	}

	// check that message with this nonce does not already exist
	_, found := pool.addressNonces[newAddressNonce(message)]
	if found {
		return errors.Errorf("message pool contains message with same actor and nonce but different cid")
	}

	// check that the message is likely to succeed in processing
	return pool.validator.ValidateSignedMessageSyntax(ctx, message)
}
