package core

import (
	"context"
	"sync"

	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/metrics"
	"github.com/filecoin-project/go-filecoin/types"
)

var mpSize = metrics.NewInt64Gauge("message_pool_size", "The size of the message pool")

// MaxMessagePoolSize is the maximum number of pending messages will will allow in the message pool at any time
const MaxMessagePoolSize = 10000

// MessageTimeOut is the number of tipsets we should receive before timing out messages
const MessageTimeOut = 6

type timedmessage struct {
	message *types.SignedMessage
	addedAt uint64
}

// MessagePoolAPI defines an interface to api resources the message pool needs.
type MessagePoolAPI interface {
	BlockHeight() (uint64, error)
}

// MessagePoolValidator defines a validator that ensures a message can go through the pool.
type MessagePoolValidator interface {
	Validate(ctx context.Context, msg *types.SignedMessage) error
}

type addressNonce struct {
	addr  address.Address
	nonce uint64
}

func newAddressNonce(msg *types.SignedMessage) addressNonce {
	return addressNonce{addr: msg.From, nonce: uint64(msg.Nonce)}
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

	api           MessagePoolAPI
	validator     MessagePoolValidator
	pending       map[cid.Cid]*timedmessage // all pending messages
	addressNonces map[addressNonce]bool     // set of address nonce pairs used to efficiently validate duplicate nonces
}

// Add adds a message to the pool.
func (pool *MessagePool) Add(ctx context.Context, msg *types.SignedMessage) (cid.Cid, error) {
	blockTime, err := pool.api.BlockHeight()
	if err != nil {
		return cid.Undef, err
	}

	return pool.addTimedMessage(ctx, &timedmessage{message: msg, addedAt: blockTime})
}

// An error coming out of addTimedMessage probably means the message failed to validate,
// but it could indicate a more serious problem with the system.
func (pool *MessagePool) addTimedMessage(ctx context.Context, msg *timedmessage) (cid.Cid, error) {
	pool.lk.Lock()
	defer pool.lk.Unlock()

	c, err := msg.message.Cid()
	if err != nil {
		return cid.Undef, errors.Wrap(err, "failed to create CID")
	}

	// ignore message prior to validation if it is already in pool
	_, found := pool.pending[c]
	if found {
		return c, nil
	}

	if err = pool.validateMessage(ctx, msg.message); err != nil {
		return cid.Undef, errors.Wrap(err, "validation error adding message to pool")
	}

	pool.pending[c] = msg
	pool.addressNonces[newAddressNonce(msg.message)] = true
	mpSize.Set(ctx, int64(len(pool.pending)))
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
func (pool *MessagePool) Remove(c cid.Cid) {
	pool.lk.Lock()
	defer pool.lk.Unlock()

	msg, ok := pool.pending[c]
	if ok {
		delete(pool.addressNonces, newAddressNonce(msg.message))
		delete(pool.pending, c)
	}
	mpSize.Set(context.TODO(), int64(len(pool.pending)))
}

// NewMessagePool constructs a new MessagePool.
func NewMessagePool(api MessagePoolAPI, validator MessagePoolValidator) *MessagePool {
	return &MessagePool{
		api:           api,
		validator:     validator,
		pending:       make(map[cid.Cid]*timedmessage),
		addressNonces: make(map[addressNonce]bool),
	}
}

// UpdateMessagePool brings the message pool into the correct state after
// we accept a new block. It removes messages from the pool that are
// found in the newly adopted chain and adds back those from the removed
// chain (if any) that do not appear in the new chain. We think
// that the right model for keeping the message pool up to date is
// to think about it like a garbage collector.
func (pool *MessagePool) UpdateMessagePool(ctx context.Context, store chain.BlockProvider, oldHead, newHead types.TipSet) error {
	oldBlocks, newBlocks, err := CollectBlocksToCommonAncestor(ctx, store, oldHead, newHead)
	if err != nil {
		return err
	}

	// Add all message from the old blocks to the message pool, so they can be mined again.
	for _, blk := range oldBlocks {
		for _, msg := range blk.Messages {
			_, err = pool.addTimedMessage(ctx, &timedmessage{message: msg, addedAt: uint64(blk.Height)})
			if err != nil {
				log.Info(err)
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
	for _, cid := range pool.messagesToTimeOut(minimumHeight) {
		pool.Remove(cid)
	}

	return nil
}

// identify all messages that need to be timed out
func (pool *MessagePool) messagesToTimeOut(minimumHeight uint64) []cid.Cid {
	pool.lk.RLock()
	defer pool.lk.RUnlock()

	cids := []cid.Cid{}
	for cid, msg := range pool.pending {
		if msg.addedAt < minimumHeight {
			cids = append(cids, cid)
		}
	}
	return cids
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

// validateMessage validates that too many messages aren't added to the pool and the ones that are
// have a high probability of making it through processing.
func (pool *MessagePool) validateMessage(ctx context.Context, message *types.SignedMessage) error {
	if len(pool.pending) >= MaxMessagePoolSize {
		return errors.Errorf("message pool is full (%d messages)", MaxMessagePoolSize)
	}

	// check that message with this nonce does not already exist
	_, found := pool.addressNonces[newAddressNonce(message)]
	if found {
		return errors.Errorf("message pool contains message with same actor and nonce but different cid")
	}

	// check that the message is likely to succeed in processing
	return pool.validator.Validate(ctx, message)
}
