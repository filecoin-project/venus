package message

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/metrics"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

var (
	mqSizeGa   = metrics.NewInt64Gauge("message_queue_size", "The size of the message queue")
	mqOldestGa = metrics.NewInt64Gauge("message_queue_oldest", "The age of the oldest message in the queue or zero when empty")
	mqExpireCt = metrics.NewInt64Counter("message_queue_expire", "The number messages expired from the queue")
)

// Queue stores an ordered list of messages (per actor) and enforces that their nonces form a contiguous sequence.
// Each message is associated with a "stamp" (an opaque integer), and the queue supports expiring any list
// of messages where the first message has a stamp below some threshold. The relative order of stamps in a queue is
// not enforced.
// A message queue is intended to record outbound messages that have been transmitted but not yet appeared in a block,
// where the stamp could be block height.
// Queue is safe for concurrent access.
type Queue struct {
	lk sync.RWMutex
	// Message queues keyed by sending actor address, in nonce order
	queues map[address.Address][]*Queued
}

// Queued is a message an the stamp it was enqueued with.
type Queued struct {
	Msg   *types.SignedMessage
	Stamp uint64
}

// NewQueue constructs a new, empty queue.
func NewQueue() *Queue {
	return &Queue{
		queues: make(map[address.Address][]*Queued),
	}
}

// Enqueue appends a new message for an address. If the queue already contains any messages for
// from same address, the new message's nonce must be exactly one greater than the largest nonce
// present.
func (mq *Queue) Enqueue(ctx context.Context, msg *types.SignedMessage, stamp uint64) error {
	defer func() {
		mqSizeGa.Set(ctx, mq.Size())
		mqOldestGa.Set(ctx, int64(mq.Oldest()))
	}()

	mq.lk.Lock()
	defer mq.lk.Unlock()

	q := mq.queues[msg.Message.From]
	if len(q) > 0 {
		nextNonce := q[len(q)-1].Msg.Message.CallSeqNum + 1
		if msg.Message.CallSeqNum != nextNonce {
			return errors.Errorf("Invalid nonce in %d in enqueue, expected %d", msg.Message.CallSeqNum, nextNonce)
		}
	}
	mq.queues[msg.Message.From] = append(q, &Queued{msg, stamp})
	return nil
}

// Requeue prepends a message for an address. If the queue already contains any messages from the
// same address, the message's nonce must be exactly one *less than* the smallest nonce present.
func (mq *Queue) Requeue(ctx context.Context, msg *types.SignedMessage, stamp uint64) error {
	defer func() {
		mqSizeGa.Set(ctx, mq.Size())
		mqOldestGa.Set(ctx, int64(mq.Oldest()))
	}()

	mq.lk.Lock()
	defer mq.lk.Unlock()

	q := mq.queues[msg.Message.From]
	if len(q) > 0 {
		prevNonce := q[0].Msg.Message.CallSeqNum - 1
		if msg.Message.CallSeqNum != prevNonce {
			return errors.Errorf("Invalid nonce %d in requeue, expected %d", msg.Message.CallSeqNum, prevNonce)
		}
	}
	mq.queues[msg.Message.From] = append([]*Queued{{msg, stamp}}, q...)
	return nil
}

// RemoveNext removes and returns a single message from the queue, if it bears the expected nonce value, with found = true.
// Returns found = false if the queue is empty or the expected nonce is less than any in the queue for that address
// (indicating the message had already been removed).
// Returns an error if the expected nonce is greater than the smallest in the queue.
// The caller may wish to check that the returned message is equal to that expected (not just in nonce value).
func (mq *Queue) RemoveNext(ctx context.Context, sender address.Address, expectedNonce uint64) (msg *types.SignedMessage, found bool, err error) {
	defer func() {
		mqSizeGa.Set(ctx, mq.Size())
		mqOldestGa.Set(ctx, int64(mq.Oldest()))
	}()

	mq.lk.Lock()
	defer mq.lk.Unlock()

	q := mq.queues[sender]
	if len(q) > 0 {
		head := q[0]
		if expectedNonce == uint64(head.Msg.Message.CallSeqNum) {
			mq.queues[sender] = q[1:] // pop the head
			msg = head.Msg
			found = true
		} else if expectedNonce > uint64(head.Msg.Message.CallSeqNum) {
			err = errors.Errorf("Next message for %s has nonce %d, expected %d", sender, head.Msg.Message.CallSeqNum, expectedNonce)
		}
		// else expected nonce was before the head of the queue, already removed
	}
	return
}

// Clear removes all messages for a single sender address.
// Returns whether the queue was non-empty before being cleared.
func (mq *Queue) Clear(ctx context.Context, sender address.Address) bool {
	defer func() {
		mqSizeGa.Set(ctx, mq.Size())
		mqOldestGa.Set(ctx, int64(mq.Oldest()))
	}()

	mq.lk.Lock()
	defer mq.lk.Unlock()

	q := mq.queues[sender]
	delete(mq.queues, sender)
	return len(q) > 0
}

// ExpireBefore clears the queue of any sender where the first message in the queue has a stamp less than `stamp`.
// Returns a map containing any expired address queues.
func (mq *Queue) ExpireBefore(ctx context.Context, stamp uint64) map[address.Address][]*types.SignedMessage {
	defer func() {
		mqSizeGa.Set(ctx, mq.Size())
		mqOldestGa.Set(ctx, int64(mq.Oldest()))
	}()

	mq.lk.Lock()
	defer mq.lk.Unlock()

	expired := make(map[address.Address][]*types.SignedMessage)

	for sender, q := range mq.queues {
		if len(q) > 0 && q[0].Stamp < stamp {

			// record the number of messages to be expired
			mqExpireCt.Inc(ctx, int64(len(q)))
			for _, m := range q {
				expired[sender] = append(expired[sender], m.Msg)
			}

			mq.queues[sender] = []*Queued{}
		}
	}
	return expired
}

// LargestNonce returns the largest nonce of any message in the queue for an address.
// If the queue for the address is empty, returns (0, false).
func (mq *Queue) LargestNonce(sender address.Address) (largest uint64, found bool) {
	mq.lk.RLock()
	defer mq.lk.RUnlock()
	q := mq.queues[sender]
	if len(q) > 0 {
		return uint64(q[len(q)-1].Msg.Message.CallSeqNum), true
	}
	return 0, false
}

// Queues returns the addresses associated with each non-empty queue.
// The order of returned addresses is neither defined nor stable.
func (mq *Queue) Queues() []address.Address {
	mq.lk.RLock()
	defer mq.lk.RUnlock()

	keys := make([]address.Address, len(mq.queues))
	i := 0
	for k := range mq.queues {
		keys[i] = k
		i++
	}
	return keys
}

// Size returns the total number of messages in the Queue.
func (mq *Queue) Size() int64 {
	mq.lk.RLock()
	defer mq.lk.RUnlock()

	var l int64
	for _, q := range mq.queues {
		l += int64(len(q))
	}
	return l
}

// Oldest returns the oldest message stamp in the Queue.
// Oldest returns 0 if the queue is empty.
// Exported for testing only.
func (mq *Queue) Oldest() (oldest uint64) {
	mq.lk.Lock()
	defer mq.lk.Unlock()

	if len(mq.queues) == 0 {
		return 0
	}

	// max uint64 value
	oldest = 1<<64 - 1
	for _, qm := range mq.queues {
		for _, m := range qm {
			if m.Stamp < oldest {
				oldest = m.Stamp
			}
		}
	}
	return oldest
}

// List returns a copy of the list of messages queued for an address.
func (mq *Queue) List(sender address.Address) []*Queued {
	mq.lk.RLock()
	defer mq.lk.RUnlock()
	q := mq.queues[sender]
	out := make([]*Queued, len(q))
	for i, qm := range q {
		out[i] = &Queued{}
		*out[i] = *qm
	}
	return out
}
