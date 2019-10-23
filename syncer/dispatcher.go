package syncer

import (
	"container/heap"
	"context"

	logging "github.com/ipfs/go-log"

	"github.com/filecoin-project/go-filecoin/block"
)

var log = logging.Logger("sync.dispatch")

// This is the size of the channel buffer used for receiving targets from
// producers.
const incomingBufferSize = 5

// syncer is the interface of the logic syncing incoming chains
type syncer interface {
	HandleNewTipSet(context.Context, *block.ChainInfo, bool) error
}

// NewDispatcher creates a new syncing dispatcher.
func NewDispatcher(catchupSyncer syncer) *Dispatcher {
	return &Dispatcher{
		workQueue:     NewTargetQueue(),
		catchupSyncer: catchupSyncer,
		incoming:      make(chan Target, incomingBufferSize),
		control:       make(chan interface{}),
		registeredCb:  func(t Target) {},
	}
}

// cbMessage registers a user callback to be fired following every successful
// sync.
type cbMessage struct {
	cb func(Target)
}

// Dispatcher receives, sorts and dispatches targets to the syncer to control
// chain syncing.
//
// New targets arrive over the incoming channel. The dispatcher then puts them
// into the workQueue which sorts them by their claimed chain height. The
// dispatcher pops the highest priority target from the queue and then attempts
// to sync the target using its internal catchupSyncer.
//
// The dispatcher has a simple control channel. It reads this for external
// controls. Currently there is only one kind of control message.  It registers
// a callback that the dispatcher will call after every non-erroring sync.
type Dispatcher struct {
	// The following fields handle syncer target dispatch and processing
	// The dispatcher maintains a targeting system for determining the
	// current best syncing target
	// workQueue is a priority queue of target chain heads that should be
	// synced
	workQueue *TargetQueue
	// incoming is the queue of incoming sync targets to the dispatcher.
	// The dispatcher relies on a single reader pulling from this.  Don't add
	// another reader without care.
	incoming chan Target
	// catchupSyncer is used for dispatching sync targets for chain heads
	// during the CHAIN_CATCHUP mode of operation
	catchupSyncer syncer

	// The following fields allow outside processes to issue commands to
	// the dispatcher, for example to synchronize with it or inspect state
	// registeredCb is a callback registered over the control channel.  It
	// is caleld after every successful sync.
	registeredCb func(Target)
	// control is a queue of control messages not yet processed
	control chan interface{}

	// The following fields are diagnostics maintained by the dispatcher
	// syncTargetCount tracks the total number of sync targets dispatched
	// to and processed without error by the syncer.  We do not handle
	// overflows.
	syncTargetCount uint64
}

// ReceiveHello handles chain information from bootstrap peers.
func (d *Dispatcher) ReceiveHello(ci *block.ChainInfo) error { return d.receive(ci) }

// ReceiveOwnBlock handles chain info from a node's own mining system
func (d *Dispatcher) ReceiveOwnBlock(ci *block.ChainInfo) error { return d.receive(ci) }

// ReceiveGossipBlock handles chain info from new blocks sent on pubsub
func (d *Dispatcher) ReceiveGossipBlock(ci *block.ChainInfo) error { return d.receive(ci) }

func (d *Dispatcher) receive(ci *block.ChainInfo) error {
	d.incoming <- Target{ChainInfo: *ci}
	return nil
}

// Start launches the business logic for the syncing subsystem.
// It reads syncing targets from the incoming queue, sorts them, and dispatches
// them to the appropriate syncer.
func (d *Dispatcher) Start(syncingCtx context.Context) {
	go func() {
		var last *Target
		for {
			// Handle shutdown
			select {
			case <-syncingCtx.Done():
				return
			default:
			}

			// Handle control signals
			select {
			case ctrl := <-d.control:
				d.receiveCtrl(ctrl)
			default:
			}

			// Handle incoming targets
			var produced []Target
			if last != nil {
				produced = append(produced, *last)
				last = nil
			}
			select {
			case first := <-d.incoming:
				produced = append(produced, first)
				produced = append(produced, d.drainIncoming()...)
			default:
			}
			// Sort new targets by putting on work queue.
			for _, syncTarget := range produced {
				d.workQueue.Push(syncTarget)
			}

			// Check for work to do
			syncTarget, popped := d.workQueue.Pop()
			if popped {
				// Do work
				err := d.catchupSyncer.HandleNewTipSet(syncingCtx, &syncTarget.ChainInfo, true)
				if err != nil {
					log.Info("sync request could not complete: %s", err)
				}
				d.syncTargetCount++
				d.registeredCb(syncTarget)
			} else {
				// No work left, block until something shows up
				select {
				case extra := <-d.incoming:
					last = &extra
				}
			}
		}
	}()
}

// drainProduced reads all values within the incoming channel buffer at time
// of calling without blocking.  It reads at most incomingBufferSize.
func (d *Dispatcher) drainIncoming() []Target {
	// drain channel. Note this relies on a single reader of the incoming
	// channel to avoid blocking.
	n := len(d.incoming)
	produced := make([]Target, n)
	for i := 0; i < n; i++ {
		produced[i] = <-d.incoming
	}
	return produced
}

// RegisterCallback registers a callback on the dispatcher that
// will fire after every successful target sync.
func (d *Dispatcher) RegisterCallback(cb func(Target)) {
	d.control <- cbMessage{cb: cb}
}

// receiveCtrl takes a control message, determines its type, and performs the
// specified action.
func (d *Dispatcher) receiveCtrl(i interface{}) {
	// Using interfaces is overkill for now but is the way to make this
	// extensible.  (Delete this comment if we add more than one control)
	switch msg := i.(type) {
	case cbMessage:
		d.registeredCb = msg.cb
	default:
		// We don't know this type, log and ignore
		log.Info("dispatcher control can not handle type %T", msg)
	}
}

// Target tracks a logical request of the syncing subsystem to run a
// syncing job against given inputs. Targets are created by the
// Dispatcher by inspecting incoming hello messages from bootstrap peers
// and gossipsub block propagations.
type Target struct {
	block.ChainInfo
}

// TargetQueue orders dispatcher syncRequests by the underlying targetQueue's
// policy.  It wraps the targetQueue to prevent panics during normal operation.
// It also filters the targetQueue so that it always contains targets with
// unique chain heads.
//
// It is not threadsafe.
type TargetQueue struct {
	q         targetQueue
	targetSet map[string]struct{}
}

// NewTargetQueue returns a new target queue with an initialized targetQueue
func NewTargetQueue() *TargetQueue {
	rq := make(targetQueue, 0)
	heap.Init(&rq)
	return &TargetQueue{
		q:         rq,
		targetSet: make(map[string]struct{}),
	}
}

// Push adds a sync request to the target queue.
func (tq *TargetQueue) Push(req Target) {
	// If already in queue drop quickly
	if _, inQ := tq.targetSet[req.ChainInfo.Head.String()]; inQ {
		return
	}
	heap.Push(&tq.q, req)
	tq.targetSet[req.ChainInfo.Head.String()] = struct{}{}

	return
}

// Pop removes and returns the highest priority syncing target. If there is
// nothing in the queue the second argument returns false
func (tq *TargetQueue) Pop() (Target, bool) {
	if tq.Len() == 0 {
		return Target{}, false
	}
	req := heap.Pop(&tq.q).(Target)
	popKey := req.ChainInfo.Head.String()
	delete(tq.targetSet, popKey)
	return req, true
}

// Len returns the number of targets in the queue.
func (tq *TargetQueue) Len() int {
	return tq.q.Len()
}

// targetQueue orders targets by a policy.
// The current simple policy is to order syncing requests by claimed chain
// height.
//
// targetQueue can panic so it shouldn't be used unwrapped
type targetQueue []Target

// Heavily inspired by https://golang.org/pkg/container/heap/
func (rq targetQueue) Len() int { return len(rq) }

func (rq targetQueue) Less(i, j int) bool {
	// We want Pop to give us the highest priority so we use greater than
	return rq[i].Height > rq[j].Height
}

func (rq targetQueue) Swap(i, j int) {
	rq[i], rq[j] = rq[j], rq[i]
}

func (rq *targetQueue) Push(x interface{}) {
	syncReq := x.(Target)
	*rq = append(*rq, syncReq)
}

func (rq *targetQueue) Pop() interface{} {
	old := *rq
	n := len(old)
	item := old[n-1]
	*rq = old[0 : n-1]
	return item
}
