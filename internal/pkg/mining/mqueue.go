package mining

import (
	"bytes"
	"container/heap"
	"sort"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

// MessageQueue is a priority queue of messages from different actors. Messages are ordered
// by decreasing gas price, subject to the constraint that messages from a single actor are
// always in increasing nonce order.
// All messages for a queue are inserted at construction, after which messages may only
// be popped.
// Potential improvements include:
// - deprioritising messages after a gap in nonce value, which can never be mined (see Ethereum)
// - attempting to pack messages into a fixed gas limit (i.e. 0/1 knapsack subject to nonce ordering),
//   see https://en.wikipedia.org/wiki/Knapsack_problem
type MessageQueue struct {
	// A heap of nonce-ordered queues, one per sender.
	senderQueues queueHeap
}

// NewMessageQueue allocates and initializes a message queue.
func NewMessageQueue(msgs []*types.SignedMessage) MessageQueue {
	// Group messages by sender.
	bySender := make(map[address.Address]nonceQueue)
	for _, m := range msgs {
		bySender[m.Message.From] = append(bySender[m.Message.From], m)
	}

	// Order each sender queue by nonce and initialize heap structure.
	addrHeap := make(queueHeap, len(bySender))
	heapIdx := 0
	for _, nq := range bySender {
		sort.Slice(nq, func(i, j int) bool { return nq[i].Message.CallSeqNum < nq[j].Message.CallSeqNum })
		addrHeap[heapIdx] = nq
		heapIdx++
	}
	heap.Init(&addrHeap)

	return MessageQueue{addrHeap}
}

// Empty tests whether the queue is empty.
func (mq *MessageQueue) Empty() bool {
	return len(mq.senderQueues) == 0
}

// Pop removes and returns the next message from the queue, returning (nil, false) if none remain.
func (mq *MessageQueue) Pop() (*types.SignedMessage, bool) {
	if len(mq.senderQueues) == 0 {
		return nil, false
	}
	// Select actor with best gas price.
	bestQueue := &mq.senderQueues[0]

	// Pop first message off that actor's queue
	msg := (*bestQueue)[0]
	if len(*bestQueue) == 1 {
		// If the actor's queue will become empty, remove it from the heap.
		heap.Pop(&mq.senderQueues)
	} else {
		// If the actor's queue still has elements, remove the first and relocate the queue in the heap
		// according to the gas price of its next message.
		*bestQueue = (*bestQueue)[1:]
		heap.Fix(&mq.senderQueues, 0)
	}
	return msg, true
}

// Drain removes and returns all messages in a slice.
func (mq *MessageQueue) Drain() []*types.SignedMessage {
	var out []*types.SignedMessage
	for msg, hasMore := mq.Pop(); hasMore; msg, hasMore = mq.Pop() {
		out = append(out, msg)
	}
	return out
}

// A slice of messages ordered by CallSeqNum (for a single sender).
type nonceQueue []*types.SignedMessage

// Implements heap.Interface to hold a priority queue of nonce-ordered queues, one per sender.
// Heap priority is given by the gas price of the first message for each queue.
// Each sender queue is expected to be ordered by increasing nonce.
// Implementation is simplified from https://golang.org/pkg/container/heap/#example__priorityQueue.
type queueHeap []nonceQueue

func (pq queueHeap) Len() int { return len(pq) }

// Less implements Heap.Interface.Less to compare items on gas price and sender address.
func (pq queueHeap) Less(i, j int) bool {
	delta := pq[i][0].Message.GasPrice.Sub(pq[j][0].Message.GasPrice)
	if !delta.Equal(types.ZeroAttoFIL) {
		// We want Pop to give us the highest gas price, so use GreaterThan.
		return delta.GreaterThan(types.ZeroAttoFIL)
	}
	// Secondarily order by address to give a stable ordering.
	return bytes.Compare(pq[i][0].Message.From.Bytes(), pq[j][0].Message.From.Bytes()) < 0
}

func (pq queueHeap) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *queueHeap) Push(x interface{}) {
	item := x.(nonceQueue)
	*pq = append(*pq, item)
}

func (pq *queueHeap) Pop() interface{} {
	n := len(*pq)
	item := (*pq)[n-1]
	*pq = (*pq)[0 : n-1]
	return item
}
