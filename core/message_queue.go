package core

import (
	"sync"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
)

// MessageQueue stores an ordered list of messages (per actor) and enforces that their nonces form a contiguous sequence.
// Each message is associated with a "stamp" (an opaque integer), and the queue supports expiring any list
// of messages where the first message has a stamp below some threshold. The relative order of stamps in a queue is
// not enforced.
// A message queue is intended to record outbound messages that have been transmitted but not yet appeared in a block,
// where the stamp could be block height.
// MessageQueue is safe for concurrent access.
type MessageQueue struct {
	lk sync.RWMutex
	// Message queues keyed by sending actor address, in nonce order
	queues map[address.Address][]*queuedMessage
}

type queuedMessage struct {
	msg   *types.SignedMessage
	stamp uint64
}

// NewMessageQueue constructs a new, empty queue.
func NewMessageQueue() *MessageQueue {
	return &MessageQueue{
		queues: make(map[address.Address][]*queuedMessage),
	}
}

// Enqueue appends a new message for an address. If the queue already contains any messages for
// from same address, the new message's nonce must be exactly one greater than the largest nonce
// present.
func (mq *MessageQueue) Enqueue(msg *types.SignedMessage, stamp uint64) error {
	mq.lk.Lock()
	defer mq.lk.Unlock()

	q := mq.queues[msg.From]
	if len(q) > 0 {
		nextNonce := q[len(q)-1].msg.Nonce + 1
		if msg.Nonce != nextNonce {
			return errors.Errorf("Invalid nonce %d, expected %d", msg.Nonce, nextNonce)
		}
	}
	mq.queues[msg.From] = append(q, &queuedMessage{msg, stamp})
	return nil
}

// RemoveNext removes and returns a single message from the queue, if it bears the expected nonce value, with found = true.
// Returns found = false if the queue is empty or the expected nonce is less than any in the queue for that address
// (indicating the message had already been removed).
// Returns an error if the expected nonce is greater than the smallest in the queue.
// The caller may wish to check that the returned message is equal to that expected (not just in nonce value).
func (mq *MessageQueue) RemoveNext(sender address.Address, expectedNonce uint64) (msg *types.SignedMessage, found bool, err error) {
	mq.lk.Lock()
	defer mq.lk.Unlock()

	q := mq.queues[sender]
	if len(q) > 0 {
		head := q[0]
		if expectedNonce == uint64(head.msg.Nonce) {
			mq.queues[sender] = q[1:] // pop the head
			msg = head.msg
			found = true
		} else if expectedNonce > uint64(head.msg.Nonce) {
			err = errors.Errorf("Next message for %s has nonce %d, expected %d", sender, head.msg.Nonce, expectedNonce)
		}
		// else expected nonce was before the head of the queue, already removed
	}
	return
}

// Clear removes all messages for a single sender address.
func (mq *MessageQueue) Clear(sender address.Address) bool {
	mq.lk.Lock()
	defer mq.lk.Unlock()

	q := mq.queues[sender]
	if len(q) > 0 {
		mq.queues[sender] = []*queuedMessage{}
		return true
	}
	return false
}

// ExpireBefore clears the queue of any sender where the first message in the queue has a stamp less than `stamp`.
// Returns a map containing any expired address queues.
func (mq *MessageQueue) ExpireBefore(stamp uint64) map[address.Address][]*types.SignedMessage {
	mq.lk.Lock()
	defer mq.lk.Unlock()

	expired := make(map[address.Address][]*types.SignedMessage)

	for sender, q := range mq.queues {
		if len(q) > 0 && q[0].stamp < stamp {
			for _, m := range q {
				expired[sender] = append(expired[sender], m.msg)
			}
			mq.queues[sender] = []*queuedMessage{}
		}
	}
	return expired
}

// LargestNonce returns the largest nonce of any message in the queue for an address.
// If the queue for the address is empty, returns (0, false).
func (mq *MessageQueue) LargestNonce(sender address.Address) (largest uint64, found bool) {
	mq.lk.RLock()
	defer mq.lk.RUnlock()
	q := mq.queues[sender]
	if len(q) > 0 {
		return uint64(q[len(q)-1].msg.Nonce), true
	}
	return 0, false
}

// NextStamp returns the stamp for the next message in the queue for an address, or zero if the
// queue is empty.
func (mq *MessageQueue) NextStamp(sender address.Address) uint64 {
	mq.lk.RLock()
	defer mq.lk.RUnlock()
	q := mq.queues[sender]
	if len(q) > 0 {
		return q[0].stamp
	}
	return 0
}
