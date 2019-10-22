package syncer

import (
	"container/heap"
	"context"
	"errors"

	logging "github.com/ipfs/go-log"

	"github.com/filecoin-project/go-filecoin/block"
)

var log = logging.Logger("sync.dispatch")

var errEmptyPop = errors.New("pop on empty targetQueue")

const productionBufferSize = 5

// syncer is the interface of the logic syncing incoming chains
type syncer interface {
	HandleNewTipSet(context.Context, *block.ChainInfo, bool) error
}

// NewDispatcher creates a new syncing dispatcher.
func NewDispatcher(catchupSyncer syncer) *Dispatcher {
	return &Dispatcher{
		targetQ:           NewTargetQueue(),
		catchupSyncer:     catchupSyncer,
		productionChannel: make(chan *SyncRequest, productionBufferSize),
	}
}

// Dispatcher executes syncing requests
type Dispatcher struct {
	// The dispatcher maintains a targeting system for determining the
	// current best syncing target
	// targetQ is a priority queue of target tipsets
	targetQ *TargetQueue

	// productionChannel synchronizes adding sync requests to the dispatcher.
	// The dispatcher relies on a single reader pulling from this.  Don't add
	// another reader without care.
	productionChannel chan *SyncRequest

	// catchupSyncer is used for dispatching sync requests for chain heads
	// during the CHAIN_CATCHUP mode of operation
	catchupSyncer syncer
}

// ReceiveHello handles chain information from bootstrap peers.
func (d *Dispatcher) ReceiveHello(ci *block.ChainInfo) error { return d.receive(ci) }

// ReceiveOwnBlock handles chain info from a node's own mining system
func (d *Dispatcher) ReceiveOwnBlock(ci *block.ChainInfo) error { return d.receive(ci) }

// ReceiveGossipBlock handles chain info from new blocks sent on pubsub
func (d *Dispatcher) ReceiveGossipBlock(ci *block.ChainInfo) error { return d.receive(ci) }

func (d *Dispatcher) receive(ci *block.ChainInfo) error {
	d.productionChannel <- &SyncRequest{ChainInfo: *ci}
	return nil
}

// Start launches the business logic for the syncing subsystem.
// It reads syncing requests from the target queue and dispatches them to the
// appropriate syncer.
func (d *Dispatcher) Start(syncingCtx context.Context) {
	go func() {
		for {
			var produced []*SyncRequest
			// If there's something on the target queue: read from
			// the production queue without blocking.
			if d.targetQ.Len() != 0 {
				select {
				case first := <-d.productionChannel:
					produced = append(produced, first)
					produced = append(produced, d.drainProduced()...)
				case <-syncingCtx.Done():
					return
				default: // go straight to syncing
				}
			} else { // If there's nothing on the target queue:
				// block until we have something from production
				// queue.
				select {
				case first := <-d.productionChannel:
					produced = append(produced, first)
					produced = append(produced, d.drainProduced()...)
				case <-syncingCtx.Done():
					return
				}
			}

			// Sort outstanding requests and handle the next request
			for _, syncReq := range produced {
				d.targetQ.Push(syncReq)
			}
			syncReq, err := d.targetQ.Pop()
			if err != nil {
				// This is expected: target queue can be empty if
				// all new requests duplicate an existing one
				log.Debugf("error popping in sync dispatch: %s", err)
				continue
			}
			err = d.catchupSyncer.HandleNewTipSet(syncingCtx, &syncReq.ChainInfo, true)
			if err != nil {
				log.Infof("error running sync request %s", err)
			}
		}
	}()
}

func (d *Dispatcher) drainProduced() []*SyncRequest {
	// drain channel. Note this relies on a single reader of the production
	// channel.
	n := len(d.productionChannel)
	var produced []*SyncRequest
	for i := 0; i < n; i++ {
		next := <-d.productionChannel
		produced = append(produced, next)
	}
	return produced
}

// SyncRequest tracks a logical request of the syncing subsystem to run a
// syncing job against given inputs. syncRequests are created by the
// Dispatcher by inspecting incoming hello messages from bootstrap peers
// and gossipsub block propagations.
type SyncRequest struct {
	block.ChainInfo
	// needed by internal container/heap methods for maintaining sort
	index int
}

// rawQueue orders the dispatchers syncRequests by a policy.
// The current simple policy is to order syncing requests by claimed chain
// height.
//
// rawQueue can panic so it shouldn't be used unwrapped
type rawQueue []*SyncRequest

// Heavily inspired by https://golang.org/pkg/container/heap/
func (rq rawQueue) Len() int { return len(rq) }

func (rq rawQueue) Less(i, j int) bool {
	// We want Pop to give us the highest priority so we use greater than
	return rq[i].Height > rq[j].Height
}

func (rq rawQueue) Swap(i, j int) {
	rq[i], rq[j] = rq[j], rq[i]
	rq[i].index = j
	rq[j].index = i
}

func (rq *rawQueue) Push(x interface{}) {
	n := len(*rq)
	syncReq := x.(*SyncRequest)
	syncReq.index = n
	*rq = append(*rq, syncReq)
}

func (rq *rawQueue) Pop() interface{} {
	old := *rq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*rq = old[0 : n-1]
	return item
}

// TargetQueue orders dispatcher syncRequests by the underlying rawQueue's
// policy.
//
// It is not threadsafe.
type TargetQueue struct {
	q         rawQueue
	targetSet map[string]struct{}
}

// NewTargetQueue returns a new target queue with an initialized rawQueue
func NewTargetQueue() *TargetQueue {
	rq := make(rawQueue, 0)
	heap.Init(&rq)
	return &TargetQueue{
		q:         rq,
		targetSet: make(map[string]struct{}),
	}
}

// Push adds a sync request to the target queue.
func (tq *TargetQueue) Push(req *SyncRequest) {
	// If already in queue drop quickly
	if _, inQ := tq.targetSet[req.ChainInfo.Head.String()]; inQ {
		return
	}
	heap.Push(&tq.q, req)
	tq.targetSet[req.ChainInfo.Head.String()] = struct{}{}

	return
}

// Pop removes and returns the highest priority syncing target.
func (tq *TargetQueue) Pop() (*SyncRequest, error) {
	if tq.Len() == 0 {
		return nil, errEmptyPop
	}
	req := heap.Pop(&tq.q).(*SyncRequest)
	popKey := req.ChainInfo.Head.String()
	delete(tq.targetSet, popKey)
	return req, nil
}

// Len returns the number of targets in the queue.
func (tq *TargetQueue) Len() int {
	return tq.q.Len()
}
