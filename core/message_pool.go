package core

import (
	"context"
	"sync"

	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/metrics"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

var mpSize = metrics.NewInt64Gauge("message_pool_size", "The size of the message pool")

// MaxMessagePoolSize is the maximum number of pending messages will will allow in the message pool at any time
const MaxMessagePoolSize = 10000

// MaxNonceGap is the maximum nonce of a message past the last received on chain
const MaxNonceGap = 100

// MessageTimeOut is the number of tipsets we should receive before timing out messages
const MessageTimeOut = 6

var (
	ErrMessagePoolFull  = errors.New("message pool is full")
	ErrNonceGapExceeded = errors.New("message nonce is too much greater than highest nonce on chain")
	ErrDuplicateNonce   = errors.New("different message with same from address and nonce is already in pool")
)

type timedmessage struct {
	message *types.SignedMessage
	addedAt uint64
}

// MessagePoolAPI defines a interface api resources the message pool needs.
type MessagePoolAPI interface {
	BlockHeight() (uint64, error)
	LatestState(ctx context.Context) (state.Tree, error)
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
	pending       map[cid.Cid]*timedmessage // all pending messages
	addressNonces map[addressNonce]bool
}

// Add adds a message to the pool.
func (pool *MessagePool) Add(ctx context.Context, msg *types.SignedMessage) (cid.Cid, error) {
	blockTime, err := pool.api.BlockHeight()
	if err != nil {
		return cid.Undef, err
	}

	return pool.addTimedMessage(ctx, &timedmessage{message: msg, addedAt: blockTime})
}

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

	err = pool.ValidateMessage(ctx, msg.message)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "validation error adding message to pool")
	}

	pool.pending[c] = msg
	pool.addressNonces[newAddressNonce(msg.message)] = true
	mpSize.Set(context.TODO(), int64(len(pool.pending)))
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

	msg, ok := pool.pending[c]
	if ok {
		delete(pool.addressNonces, newAddressNonce(msg.message))
	}
	delete(pool.pending, c)
	mpSize.Set(context.TODO(), int64(len(pool.pending)))
}

// NewMessagePool constructs a new MessagePool.
func NewMessagePool(api MessagePoolAPI) *MessagePool {
	return &MessagePool{
		api:           api,
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

// ValidateMessage in message pool is a mechanism for preventing memory based DoS attacks.
// As such, it will often fail silently.
func (pool *MessagePool) ValidateMessage(ctx context.Context, message *types.SignedMessage) error {
	if len(pool.pending) >= MaxMessagePoolSize {
		return ErrMessagePoolFull
	}

	// check that message with this nonce does not already exist
	_, found := pool.addressNonces[newAddressNonce(message)]
	if found {
		return ErrDuplicateNonce
	}

	st, err := pool.api.LatestState(ctx)
	if err != nil {
		return err
	}

	fromActor, err := st.GetActor(ctx, message.From)
	if err != nil {
		if state.IsActorNotFoundError(err) {
			fromActor = &actor.Actor{}
		} else {
			return err
		}
	}

	// check that message nonce is not too high
	if message.Nonce > fromActor.Nonce && message.Nonce-fromActor.Nonce > MaxNonceGap {
		return ErrNonceGapExceeded
	}

	if err = consensus.NewOutboundMessageValidator().Validate(ctx, message, fromActor); err != nil {
		return err
	}

	return nil
}
