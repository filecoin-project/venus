package dispatcher

import (
	"container/heap"
	"context"

	logging "github.com/ipfs/go-log"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
)

var log = logging.Logger("chainsync.dispatcher")

// DefaultInQueueSize is the size of the channel used for receiving targets from producers.
const DefaultInQueueSize = 5

// DefaultWorkQueueSize is the size of the work queue
const DefaultWorkQueueSize = 20

// dispatchSyncer is the interface of the logic syncing incoming chains
type dispatchSyncer interface {
	HandleNewTipSet(context.Context, *block.ChainInfo) error
}

// NewDispatcher creates a new syncing dispatcher with default queue sizes.
func NewDispatcher(catchupSyncer dispatchSyncer) *Dispatcher {
	return NewDispatcherWithSizes(catchupSyncer, DefaultWorkQueueSize, DefaultInQueueSize)
}

// NewDispatcherWithSizes creates a new syncing dispatcher.
func NewDispatcherWithSizes(syncer dispatchSyncer, workQueueSize, inQueueSize int) *Dispatcher {
	return &Dispatcher{
		workQueue:     NewTargetQueue(),
		workQueueSize: workQueueSize,
		syncer:        syncer,
		incoming:      make(chan Target, inQueueSize),
		control:       make(chan interface{}, 1),
		registeredCb:  func(t Target) {},
	}
}

// cbMessage registers a user callback to be fired following every successful
// sync.
type cbMessage struct {
	cb func(Target)
}

// Dispatcher receives, sorts and dispatches targets to the catchupSyncer to control
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
	// workQueue is a priority queue of target chain heads that should be
	// synced
	workQueue     *TargetQueue
	workQueueSize int
	// incoming is the queue of incoming sync targets to the dispatcher.
	incoming chan Target
	// catchupSyncer is used for dispatching sync targets for chain heads
	// during the CHAIN_CATCHUP mode of operation
	syncer dispatchSyncer

	// registeredCb is a callback registered over the control channel.  It
	// is called after every successful sync.
	registeredCb func(Target)
	// control is a queue of control messages not yet processed.
	control chan interface{}

	// syncTargetCount counts the number of successful syncs.
	syncTargetCount uint64
}

// SendHello handles chain information from bootstrap peers.
func (d *Dispatcher) SendHello(ci *block.ChainInfo) error { return d.enqueue(ci) }

// SendOwnBlock handles chain info from a node's own mining system
func (d *Dispatcher) SendOwnBlock(ci *block.ChainInfo) error { return d.enqueue(ci) }

// SendGossipBlock handles chain info from new blocks sent on pubsub
func (d *Dispatcher) SendGossipBlock(ci *block.ChainInfo) error { return d.enqueue(ci) }

func (d *Dispatcher) enqueue(ci *block.ChainInfo) error {
	d.incoming <- Target{ChainInfo: *ci}
	return nil
}

// Start launches the business logic for the syncing subsystem.
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
				d.processCtrl(ctrl)
			default:
			}

			// Handle incoming targets
			var ws []Target
			if last != nil {
				ws = append(ws, *last)
				last = nil
			}
			select {
			case first := <-d.incoming:
				ws = append(ws, first)
				ws = append(ws, d.drainIncoming()...)
			default:
			}
			for _, syncTarget := range ws {
				// Drop targets we don't have room for
				if d.workQueue.Len() >= d.workQueueSize {
					break
				}
				// Sort new targets by putting on work queue.
				d.workQueue.Push(syncTarget)
			}

			// Check for work to do
			syncTarget, popped := d.workQueue.Pop()
			if popped {
				// Do work
				err := d.syncer.HandleNewTipSet(syncingCtx, &syncTarget.ChainInfo)
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

func (d *Dispatcher) drainIncoming() []Target {
	// drainProduced reads all values within the incoming channel buffer at time
	// of calling without blocking.  It reads at most incomingBufferSize.
	//
	// Note: this relies on a single reader of the incoming channel to
	// avoid blocking.
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

func (d *Dispatcher) processCtrl(ctrlMsg interface{}) {
	// processCtrl takes a control message, determines its type, and performs the
	// specified action.
	//
	// Using interfaces is overkill for now but is the way to make this
	// extensible.  (Delete this comment if we add more than one control)
	switch typedMsg := ctrlMsg.(type) {
	case cbMessage:
		d.registeredCb = typedMsg.cb
	default:
		// We don't know this type, log and ignore
		log.Info("dispatcher control can not handle type %T", typedMsg)
	}
}

// Target tracks a logical request of the syncing subsystem to run a
// syncing job against given inputs.
type Target struct {
	block.ChainInfo
}

// TargetQueue orders dispatcher syncRequests by the underlying `targetQueue`'s
// prioritization policy.
//
// It also filters the `targetQueue` so that it always contains targets with
// unique chain heads.
//
// It wraps the `targetQueue` to prevent panics during
// normal operation.
type TargetQueue struct {
	q         targetQueue
	targetSet map[string]struct{}
}

// NewTargetQueue returns a new target queue.
func NewTargetQueue() *TargetQueue {
	rq := make(targetQueue, 0)
	heap.Init(&rq)
	return &TargetQueue{
		q:         rq,
		targetSet: make(map[string]struct{}),
	}
}

// Push adds a sync target to the target queue.
func (tq *TargetQueue) Push(t Target) {
	// If already in queue drop quickly
	if _, inQ := tq.targetSet[t.ChainInfo.Head.String()]; inQ {
		return
	}
	heap.Push(&tq.q, t)
	tq.targetSet[t.ChainInfo.Head.String()] = struct{}{}
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
//
// The current simple policy is to order syncing requests by claimed chain
// height.
//
// `targetQueue` can panic so it shouldn't be used unwrapped
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
