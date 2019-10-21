package syncer

import (
	"container/heap"
	"context"
	"errors"
	"sync"

	logging "github.com/ipfs/go-log"

	"github.com/filecoin-project/go-filecoin/block"
)

var log = logging.Logger("sync.dispatch")

var errBadPush = errors.New("a programmer is pushing the wrong type to a TargetQueue")
var errBadPop = errors.New("a programmer is not checking targetQueue length before popping")
var errBadSet = errors.New("a programmer is not correctly maintaining the targetSet")

// syncer is the interface of the logic syncing incoming chains
type syncer interface {
	HandleNewTipSet(context.Context, *types.ChainInfo, bool) error
}

// NewDispatcher creates a new syncing dispatcher.
func NewDispatcher(catchupSyncer syncer) *Dispatcher {
	return &Dispatcher{
		targetQ:       NewTargetQueue(),
		catchupSyncer: catchupSyncer,
	}
}

// Dispatcher executes syncing requests
type Dispatcher struct {
	// The dispatcher maintains a targeting system for determining the
	// current best syncing target
	// targetQ is a priority queue of target tipsets
	targetQ *TargetQueue

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
	err := d.targetQ.Push(&SyncRequest{ChainInfo: *ci})
	if err != nil {
		return err
	}
	return nil
}

// Start launches the business logic for the syncing subsystem.
// It reads syncing requests from the target queue and dispatches them to the
// appropriate syncer.
func (d *Dispatcher) Start(syncingCtx context.Context) {
	// Loop on targetQ.Pop()
	// Pop() should block when there is nothing there
	// When we get something we should dispatch the request to the appropriate syncer
	go func() {
		for {
			// Pop() blocks until there is something to sync
			syncReq, err := d.targetQ.Pop()
			if err != nil {
				log.Errorf("error popping next sync request %s", err)
			}
			err = d.catchupSyncer.HandleNewTipSet(syncingCtx, &syncReq.ChainInfo, true)
			if err != nil {
				log.Infof("error running sync request", err)
			}
		}
	}()
}

// ActiveRequests informs of the number of sync requests currently enqueued.
func (d *Dispatcher) ActiveRequests() int {
	return d.targetQ.Len()
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
// policy. It exposes programmer errors as return values instead of panicing.
// Errors should only be returned from Push and Pop in the case of programmer
// error.
//
// All methods are threadsafe.  Concurrent pushes and pops are allowed.
// Pop is a blocking call in the case the queue is empty.
type TargetQueue struct {
	q         rawQueue
	targetSet map[string]struct{}

	// the following fields ensure thread safety
	// popMu ensures that a single popper will wait for the empty wg
	popMu sync.Mutex
	// rawMu ensures a single go-routine accesses rawQueue and
	rawMu sync.Mutex
	// empty signals when a queue is empty and no-longer empty
	empty sync.WaitGroup
}

// NewTargetQueue returns a new target queue with an initialized rawQueue
func NewTargetQueue() *TargetQueue {
	rq := make(rawQueue, 0)
	heap.Init(&rq)
	tq := &TargetQueue{
		q:         rq,
		targetSet: make(map[string]struct{}),
	}

	// Important to add to waitgroup directly on struct, waitgroup internals
	// rely on memory not moving!
	tq.empty.Add(1)
	return tq
}

// Push adds a sync request to the target queue.
func (tq *TargetQueue) Push(req *SyncRequest) (err error) {
	tq.rawMu.Lock()
	defer tq.rawMu.Unlock()
	defer func() {
		// This converts heap.Push panics to programmer errors
		if r := recover(); r != nil {
			err = errBadPush
		}
	}()
	// If already in queue drop quickly
	if _, inQ := tq.targetSet[req.ChainInfo.Head.String()]; inQ {
		return nil
	}
	heap.Push(&tq.q, req)
	if tq.q.Len() == 1 {
		// Signal that the queue has gone from empty to non-empty
		tq.empty.Done()
	}
	tq.targetSet[req.ChainInfo.Head.String()] = struct{}{}

	return nil
}

// Pop removes and returns the highest priority syncing target.
func (tq *TargetQueue) Pop() (req *SyncRequest, err error) {
	tq.popMu.Lock()
	defer tq.popMu.Unlock()
	// Wait for a non-empty queue
	tq.empty.Wait()
	tq.rawMu.Lock()
	defer tq.rawMu.Unlock()
	defer func() {
		// This converts heap.Pop panics to programmer errors
		if r := recover(); r != nil {
			req = nil
			err = errBadPop
		}
	}()
	req, err = heap.Pop(&tq.q).(*SyncRequest), nil
	if tq.q.Len() == 0 {
		// Signal that the queue has gone from non-empty to empty
		tq.empty.Add(1)
	}
	popKey := req.ChainInfo.Head.String()
	if _, inQ := tq.targetSet[popKey]; !inQ {
		return nil, errBadSet
	}
	delete(tq.targetSet, popKey)
	return req, err
}

// Len returns the number of targets in the queue.
func (tq *TargetQueue) Len() int {
	tq.rawMu.Lock()
	defer tq.rawMu.Unlock()
	return tq.q.Len()
}
